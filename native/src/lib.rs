//! Native storage boundary. Transactions and borrowed database pages never cross the ABI.
use heed::{types::Bytes, Database as LmdbTable, Env, EnvOpenOptions};
use redb::{Database, ReadableDatabase, ReadableTable, TableDefinition};
use std::{
    error::Error,
    ops::Bound,
    panic::{catch_unwind, AssertUnwindSafe},
    slice, str,
};

type Result<T> = std::result::Result<T, Box<dyn Error>>;
const TABLE: TableDefinition<&[u8], &[u8]> = TableDefinition::new("records_v1");

enum Store {
    Red(Database),
    Lightning(Env<heed::WithoutTls>, LmdbTable<'static, Bytes, Bytes>),
}

impl Store {
    fn open(path: &str, lightning: bool) -> Result<Self> {
        if lightning {
            // The Go caller owns the directory lock throughout the environment lifetime.
            let env = unsafe {
                EnvOpenOptions::new()
                    .read_txn_without_tls()
                    .map_size(if usize::BITS >= 64 {
                        1usize.checked_shl(40).unwrap()
                    } else {
                        512 << 20
                    })
                    .open(path)?
            };
            let table = env
                .create_databases_and_commit(env.write_txn()?, |opener, txn| {
                    opener
                        .create_database(txn, None)
                        .map(heed::CommitOrAbort::<_, ()>::Commit)
                })?
                .unwrap_commit();
            Ok(Self::Lightning(env, table))
        } else {
            let db = Database::builder().set_cache_size(64 << 20).create(path)?;
            let txn = db.begin_write()?;
            txn.open_table(TABLE)?;
            txn.commit()?;
            Ok(Self::Red(db))
        }
    }

    fn get(&self, primary: &[u8]) -> Result<Option<Vec<u8>>> {
        match self {
            Self::Red(db) => {
                let txn = db.begin_read()?;
                let t = txn.open_table(TABLE)?;
                let Some(index) = t.get(primary)? else {
                    return Ok(None);
                };
                let value = t.get(index.value())?.ok_or("dangling record index")?;
                Ok(Some(value.value()[1..].to_vec()))
            }
            Self::Lightning(env, t) => {
                let txn = env.read_txn()?;
                let Some(index) = t.get(&txn, primary)? else {
                    return Ok(None);
                };
                let value = t.get(&txn, index)?.ok_or("dangling record index")?;
                Ok(Some(value[1..].to_vec()))
            }
        }
    }

    // One tree holds ID -> ordered key and ordered key -> record in disjoint namespaces.
    // Removing the old order entry and writing its replacement is a single durable commit.
    fn change(&self, primary: &[u8], order: &[u8], value: &[u8]) -> Result<()> {
        match self {
            Self::Red(db) => {
                let txn = db.begin_write()?;
                {
                    let mut t = txn.open_table(TABLE)?;
                    let old = t.get(primary)?.map(|g| g.value().to_vec());
                    if let Some(old) = old.filter(|k| k != order) {
                        t.remove(old.as_slice())?;
                    }
                    if value.is_empty() {
                        t.remove(primary)?;
                    } else {
                        t.insert(primary, order)?;
                        t.insert(order, value)?;
                    }
                }
                txn.commit()?;
            }
            Self::Lightning(env, t) => {
                let mut txn = env.write_txn()?;
                let old = t.get(&txn, primary)?.map(<[u8]>::to_vec);
                if let Some(old) = old.filter(|k| k != order) {
                    t.delete(&mut txn, &old)?;
                }
                if value.is_empty() {
                    t.delete(&mut txn, primary)?;
                } else {
                    t.put(&mut txn, primary, order)?;
                    t.put(&mut txn, order, value)?;
                }
                txn.commit()?;
            }
        }
        Ok(())
    }

    fn list(
        &self,
        low: &[u8],
        high: &[u8],
        prefix: &[u8],
        visibility: u8,
        limit: usize,
    ) -> Result<Vec<u8>> {
        let mut out = vec![b'['];
        let mut count = 0;
        let mut append = |k: &[u8], v: &[u8]| -> Result<bool> {
            let id = k.get(low.len() + 8..).ok_or("invalid order key")?;
            let (published, json) = v.split_first().ok_or("invalid record")?;
            if (visibility == 2 || *published == visibility) && id.starts_with(prefix) {
                if count > 0 {
                    out.push(b',');
                }
                out.extend_from_slice(json);
                count += 1;
            }
            Ok(limit > 0 && count >= limit)
        };
        let bounds = (Bound::Included(low), Bound::Excluded(high));
        match self {
            Self::Red(db) => {
                let txn = db.begin_read()?;
                let t = txn.open_table(TABLE)?;
                for row in t.range(bounds)?.rev() {
                    let (k, v) = row?;
                    if append(k.value(), v.value())? {
                        break;
                    }
                }
            }
            Self::Lightning(env, t) => {
                let txn = env.read_txn()?;
                for row in t.rev_range(&txn, &bounds)? {
                    let (k, v) = row?;
                    if append(k, v)? {
                        break;
                    }
                }
            }
        }
        out.push(b']');
        Ok(out)
    }
}

#[repr(C)]
pub struct Buffer {
    ptr: *mut u8,
    len: usize,
}
impl Buffer {
    unsafe fn read(&self) -> &[u8] {
        if self.len == 0 {
            &[]
        } else {
            slice::from_raw_parts(self.ptr, self.len)
        }
    }
    fn write(&mut self, v: Vec<u8>) {
        // An empty Rust Vec uses a sentinel pointer. Only pass null or actual
        // foreign allocations back through cgo.
        if v.is_empty() {
            self.len = 0;
            self.ptr = std::ptr::null_mut();
            return;
        }
        let b = v.into_boxed_slice();
        self.len = b.len();
        self.ptr = Box::into_raw(b) as *mut u8;
    }
}

#[no_mangle]
pub unsafe extern "C" fn kv_free(b: Buffer) {
    if !b.ptr.is_null() {
        drop(Box::from_raw(std::ptr::slice_from_raw_parts_mut(
            b.ptr, b.len,
        )));
    }
}

fn guard(out: &mut Buffer, f: impl FnOnce() -> Result<Option<Vec<u8>>>) -> i32 {
    let err = match catch_unwind(AssertUnwindSafe(f)) {
        Ok(Ok(Some(data))) => {
            out.write(data);
            return 0;
        }
        Ok(Ok(None)) => return 1,
        Ok(Err(e)) => e.to_string(),
        Err(_) => "database operation panicked".into(),
    };
    out.write(err.into_bytes());
    -1
}

#[no_mangle]
pub unsafe extern "C" fn kv_open(
    path: Buffer,
    lightning: bool,
    handle: &mut *mut std::ffi::c_void,
    out: &mut Buffer,
) -> i32 {
    guard(out, || {
        *handle = Box::into_raw(Box::new(Store::open(
            str::from_utf8(path.read())?,
            lightning,
        )?))
        .cast();
        Ok(Some(Vec::new()))
    })
}

#[no_mangle]
pub unsafe extern "C" fn kv_close(handle: *mut std::ffi::c_void) {
    drop(Box::from_raw(handle.cast::<Store>()));
}

#[no_mangle]
pub unsafe extern "C" fn kv_call(
    handle: *mut std::ffi::c_void,
    op: u8,
    a: Buffer,
    b: Buffer,
    c: Buffer,
    visibility: u8,
    limit: usize,
    out: &mut Buffer,
) -> i32 {
    guard(out, || {
        let store = &*handle.cast::<Store>();
        match op {
            0 => store.get(a.read()),
            1 => {
                store.change(a.read(), b.read(), c.read())?;
                Ok(Some(Vec::new()))
            }
            2 => Ok(Some(store.list(
                a.read(),
                b.read(),
                c.read(),
                visibility,
                limit,
            )?)),
            _ => Err("invalid database operation".into()),
        }
    })
}
