// Public reading works without JavaScript. Feature-detect browser agent tools.
(() => {
  const state = JSON.parse(document.querySelector("#state").textContent);
  const api = document.modelContext;
  if (!api?.registerTool) return;
  for (const tool of state.tools) {
    api.registerTool({
      name: tool.name,
      description: state.messages[tool.name],
      inputSchema: tool.inputSchema,
      async execute(input) {
        const response = await fetch("/api/v1/" + tool.name, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            "X-Th0ughts-Request": "1",
          },
          body: JSON.stringify(input),
        });
        const result = await response.json();
        return {
          content: [{ type: "text", text: JSON.stringify(result) }],
          isError: !response.ok,
        };
      },
    });
  }
})();

for (const el of document.querySelectorAll("time[datetime]")) {
  const date = new Date(el.dateTime),
    locale = document.documentElement.lang;
  el.title = new Intl.DateTimeFormat(locale, {
    dateStyle: "long",
    timeStyle: "short",
  }).format(date);
  const minutes = Math.floor((Date.now() - date.getTime()) / 60000);
  if (minutes >= 0 && minutes < 1440) {
    el.textContent = new Intl.RelativeTimeFormat(locale, {
      numeric: "auto",
    }).format(
      minutes < 60 ? -minutes : -Math.floor(minutes / 60),
      minutes < 60 ? "minute" : "hour",
    );
  }
}

const picker = document.querySelector(".language-picker");
if (picker) {
  document.addEventListener("click", (event) => {
    if (picker.open && !picker.contains(event.target)) picker.open = false;
  });
  picker.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && picker.open) {
      picker.open = false;
      picker.querySelector("summary").focus();
    }
  });
}
