#!/usr/bin/env node
import { accessSync, constants, existsSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const VERSION = "0.1.0";
const CONFIG_FILE = ".devctl.yml";
const REQUIRED_TOOLS = ["multipass", "ansible-playbook", "ssh", "ssh-add", "infisical", "git"];

function printHelp() {
  console.log(`devctl ${VERSION}

Usage:
  devctl doctor [--strict]
  devctl config [--project <path>]
  devctl init
  devctl --help

Commands:
  doctor   Check host prerequisites.
  config   Print resolved project config as JSON.
  init     Create a starter .devctl.yml in the current directory.
`);
}

function parseArgs(argv) {
  const args = { _: [] };
  for (let index = 0; index < argv.length; index += 1) {
    const value = argv[index];
    if (!value.startsWith("--")) {
      args._.push(value);
      continue;
    }

    const key = value.slice(2);
    const next = argv[index + 1];
    if (next && !next.startsWith("--")) {
      args[key] = next;
      index += 1;
    } else {
      args[key] = true;
    }
  }
  return args;
}

function commandExists(command) {
  const result = spawnSync("sh", ["-lc", `command -v ${quoteShell(command)}`], {
    encoding: "utf8"
  });
  return result.status === 0;
}

function quoteShell(value) {
  return `'${String(value).replaceAll("'", "'\\''")}'`;
}

function findConfigPath(startDir = process.cwd()) {
  let current = resolve(startDir);
  while (true) {
    const candidate = join(current, CONFIG_FILE);
    if (existsSync(candidate)) return candidate;

    const parent = dirname(current);
    if (parent === current) return null;
    current = parent;
  }
}

function stripQuotes(value) {
  const trimmed = value.trim();
  if (
    (trimmed.startsWith("\"") && trimmed.endsWith("\"")) ||
    (trimmed.startsWith("'") && trimmed.endsWith("'"))
  ) {
    return trimmed.slice(1, -1);
  }
  return trimmed;
}

function coerceScalar(value) {
  const stripped = stripQuotes(value);
  if (/^-?\d+$/.test(stripped)) return Number(stripped);
  if (stripped === "true") return true;
  if (stripped === "false") return false;
  return stripped;
}

function parseSimpleYaml(content) {
  const root = {};
  let section = null;

  for (const rawLine of content.split(/\r?\n/)) {
    const withoutComment = rawLine.replace(/\s+#.*$/, "");
    if (withoutComment.trim().length === 0) continue;

    const topLevel = withoutComment.match(/^([A-Za-z0-9_-]+):\s*(.*)$/);
    if (topLevel) {
      const [, key, value] = topLevel;
      if (value.trim().length === 0) {
        root[key] = {};
        section = key;
      } else {
        root[key] = coerceScalar(value);
        section = null;
      }
      continue;
    }

    const nested = withoutComment.match(/^\s{2}([A-Za-z0-9_-]+):\s*(.*)$/);
    if (nested && section) {
      const [, key, value] = nested;
      root[section][key] = coerceScalar(value);
      continue;
    }

    throw new Error(`Unsupported YAML line: ${rawLine}`);
  }

  return root;
}

function loadConfig(args) {
  const configPath = args.project ? resolve(args.project) : findConfigPath();
  if (!configPath || !existsSync(configPath)) {
    console.error(`No ${CONFIG_FILE} found.`);
    console.error("Run: devctl init");
    process.exit(1);
  }

  accessSync(configPath, constants.R_OK);
  return {
    config: parseSimpleYaml(readFileSync(configPath, "utf8")),
    configPath
  };
}

function runDoctor(args) {
  const missing = [];

  for (const tool of REQUIRED_TOOLS) {
    if (commandExists(tool)) {
      console.log(`ok      ${tool}`);
    } else {
      console.log(`missing ${tool}`);
      missing.push(tool);
    }
  }

  if (missing.length > 0) {
    console.log("");
    console.log(`Missing tools: ${missing.join(", ")}`);
    console.log("Install them on the host, then rerun devctl doctor.");
    if (args.strict) process.exit(1);
  }
}

function runConfig(args) {
  const { config, configPath } = loadConfig(args);
  console.log(JSON.stringify({ configPath, config }, null, 2));
}

function runInit() {
  const target = join(process.cwd(), CONFIG_FILE);
  if (existsSync(target)) {
    console.error(`${CONFIG_FILE} already exists.`);
    process.exit(1);
  }

  writeFileSync(
    target,
    `org: example
project: example-app
repo: git@github.com:example/example-app.git
vm_name: example-dev
host: example-dev.test
vm_user: ubuntu
repo_dir: /home/ubuntu/workspaces/example-app
infisical:
  project_path: /example
  default_env: dev
resources:
  cpus: 4
  memory: 6G
  disk: 50G
ports:
  app: 3000
  preview: 4173
  supabase_api: 54321
  supabase_db: 54322
  supabase_studio: 54323
  mailpit: 54324
`,
    { mode: 0o644 }
  );
  console.log(`Created ${target}`);
}

const args = parseArgs(process.argv.slice(2));
const command = args._[0];

if (!command || args.help || command === "--help" || command === "-h") {
  printHelp();
  process.exit(0);
}

try {
  if (command === "doctor") runDoctor(args);
  else if (command === "config") runConfig(args);
  else if (command === "init") runInit();
  else {
    console.error(`Unknown command: ${command}`);
    printHelp();
    process.exit(1);
  }
} catch (error) {
  console.error(error instanceof Error ? error.message : String(error));
  process.exit(1);
}

