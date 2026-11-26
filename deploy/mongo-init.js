const fs = require('fs');
db.getSiblingDB('th0ughts').createUser({
  user: 'th0ughts',
  pwd: fs.readFileSync('/run/db-init/db-password','utf8').trim(),
  roles: [{ role: 'readWrite', db: 'th0ughts' }]
});
