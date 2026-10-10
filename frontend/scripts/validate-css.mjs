/** Разбор итогового CSS и проверка авторских деклараций по грамматике значений CSS. */
import { readFile, readdir } from "node:fs/promises";
import * as csstree from "css-tree";

const files = (await readdir("dist/assets")).filter((name) =>
  name.endsWith(".css"),
);
for (const file of files) {
  csstree.parse(await readFile(`dist/assets/${file}`, "utf8"), {
    positions: true,
  });
}

const source = csstree.parse(await readFile("src/styles/main.css", "utf8"), {
  positions: true,
});
const errors = [];
csstree.walk(source, {
  visit: "Declaration",
  enter(node) {
    if (
      node.property.startsWith("--") ||
      node.property.startsWith("-webkit-")
    ) {
      return;
    }

    const match = csstree.lexer.matchProperty(node.property, node.value);
    if (match.error && !csstree.generate(node.value).includes("var(")) {
      errors.push(`${node.loc.start.line}: ${match.error.message}`);
    }
  },
});
if (errors.length) {
  throw new Error(errors.join("\n"));
}

process.stdout.write(
  `CSS valid: ${files.length} compiled stylesheet(s), authored declarations checked.\n`,
);
