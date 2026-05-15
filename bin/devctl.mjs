#!/usr/bin/env node
import {
  accessSync,
  constants,
  existsSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync
} from "node:fs";
import { arch, cpus, freemem, platform, release } from "node:os";
import { dirname, join, resolve } from "node:path";
import { tmpdir } from "node:os";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const VERSION = "0.1.0";
const CONFIG_FILE = ".devctl.yml";
const REQUIRED_TOOLS = ["multipass", "ansible-playbook", "ssh", "ssh-add", "infisical", "git", "pnpm"];
const UBUNTU_IMAGE = "24.04";
const DEFAULT_ANSIBLE_PLAYBOOK = "infra/ansible/playbooks/dev-vm.yml";
const CLI_ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "..");

function printHelp() {
  console.log(`devctl ${VERSION}

Usage:
  devctl doctor [--strict]
  devctl config [--project <path>]
  devctl init
  devctl up [--project <path>] [--yes]
  devctl hosts sync [--project <path>] [--yes]
  devctl ssh config [--project <path>]
  devctl provision [--project <path>]
  devctl --help

Commands:
  doctor      Check host prerequisites.
  config      Print resolved project config as JSON.
  init        Create a starter .devctl.yml in the current directory.
  up          Create the Multipass VM if missing.
  hosts sync  Update /etc/hosts with the VM IP.
  ssh config  Print the SSH config block for the VM.
  provision   Run the Ansible dev VM playbook.
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
  if (platform() === "win32") {
    const result = spawnSync("where", [command], { encoding: "utf8" });
    return result.status === 0;
  }

  const result = spawnSync("sh", ["-lc", `command -v ${quoteShell(command)}`], {
    encoding: "utf8"
  });
  return result.status === 0;
}

function runCommand(command, args, options = {}) {
  return spawnSync(command, args, {
    encoding: "utf8",
    stdio: options.stdio ?? "pipe",
    ...options
  });
}

function runCommandChecked(command, args, options = {}) {
  const result = runCommand(command, args, options);
  if (result.status !== 0) {
    const stderr = typeof result.stderr === "string" ? result.stderr.trim() : "";
    const stdout = typeof result.stdout === "string" ? result.stdout.trim() : "";
    const detail = stderr || stdout;
    throw new Error(`${command} ${args.join(" ")} failed${detail ? `:\n${detail}` : ""}`);
  }
  return result;
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
    console.error(`Warning: no ${CONFIG_FILE} found.`);
    console.error("Run: devctl init");
    process.exit(1);
  }

  accessSync(configPath, constants.R_OK);
  return {
    config: parseSimpleYaml(readFileSync(configPath, "utf8")),
    configPath
  };
}

function requireConfigValue(config, key) {
  const value = config[key];
  if (value === undefined || value === null || value === "") {
    throw new Error(`Missing required config key: ${key}`);
  }
  return value;
}

function requireNestedConfigValue(config, section, key) {
  const value = config[section]?.[key];
  if (value === undefined || value === null || value === "") {
    throw new Error(`Missing required config key: ${section}.${key}`);
  }
  return value;
}

function getHostPlatform() {
  const nodePlatform = platform();
  const nodeRelease = release().toLowerCase();
  const isWsl = nodePlatform === "linux" && nodeRelease.includes("microsoft");

  if (nodePlatform === "darwin") return "macos";
  if (nodePlatform === "linux" && isWsl) return "wsl";
  if (nodePlatform === "linux") return "linux";
  if (nodePlatform === "win32") return "windows";
  return nodePlatform;
}

function installHintsFor(hostPlatform, missing) {
  if (missing.length === 0) return [];

  if (hostPlatform === "macos") {
    return [
      "macOS install hints:",
      missing.includes("multipass") ? "  brew install --cask multipass" : null,
      missing.includes("ansible-playbook") ? "  brew install ansible" : null,
      missing.includes("infisical") ? "  brew install infisical/get-cli/infisical" : null,
      missing.includes("pnpm") ? "  brew install pnpm" : null,
      missing.includes("git") ? "  brew install git" : null
    ].filter(Boolean);
  }

  if (hostPlatform === "linux" || hostPlatform === "wsl") {
    return [
      `${hostPlatform === "wsl" ? "WSL2" : "Linux"} install hints:`,
      missing.includes("multipass") ? "  See: https://documentation.ubuntu.com/multipass/" : null,
      missing.includes("ansible-playbook") ? "  sudo apt-get install ansible" : null,
      missing.includes("infisical") ? "  See: https://infisical.com/docs/documentation/getting-started/cli" : null,
      missing.includes("pnpm") ? "  corepack enable && corepack prepare pnpm@10.32.1 --activate" : null,
      missing.includes("git") ? "  sudo apt-get install git" : null
    ].filter(Boolean);
  }

  if (hostPlatform === "windows") {
    return [
      "Windows native is not supported for Ansible control in V1.",
      "Use WSL2, then run devctl from the WSL shell."
    ];
  }

  return [`Unsupported host platform: ${hostPlatform}`];
}

function runDoctor(args) {
  const missing = [];
  const hostPlatform = getHostPlatform();

  console.log(`host    ${hostPlatform} ${arch()}`);

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
    for (const hint of installHintsFor(hostPlatform, missing)) console.log(hint);
    if (args.strict) process.exit(1);
  }
}

function runConfig(args) {
  const { config, configPath } = loadConfig(args);
  console.log(JSON.stringify({ configPath, config }, null, 2));
}

function parseSizeToMiB(value) {
  const match = String(value).trim().match(/^(\d+(?:\.\d+)?)([KMGTP]?)B?$/i);
  if (!match) throw new Error(`Invalid size: ${value}`);

  const amount = Number(match[1]);
  const unit = match[2].toUpperCase();
  const multiplier = {
    "": 1 / (1024 * 1024),
    K: 1 / 1024,
    M: 1,
    G: 1024,
    T: 1024 * 1024,
    P: 1024 * 1024 * 1024
  }[unit];

  return Math.ceil(amount * multiplier);
}

function formatMiB(value) {
  if (value >= 1024) return `${(value / 1024).toFixed(1)}G`;
  return `${Math.round(value)}M`;
}

function confirmOrExit(message, args) {
  if (args.yes) return;

  if (!process.stdin.isTTY) {
    throw new Error(`Refusing non-interactive confirmation for: ${message}. Pass --yes.`);
  }

  const result = runCommand(
    "sh",
    [
      "-c",
      "printf '%s' \"$1\"; IFS= read -r answer; case \"$answer\" in y|Y|yes|YES) exit 0 ;; *) exit 1 ;; esac",
      "devctl-confirm",
      `${message} [y/N] `
    ],
    { stdio: "inherit" }
  );

  if (result.status === 0) return;

  console.error("Aborted.");
  process.exit(1);
}

function ensureTool(command) {
  if (!commandExists(command)) {
    throw new Error(`${command} is missing. Run: devctl doctor`);
  }
}

function getVmName(config) {
  return String(requireConfigValue(config, "vm_name"));
}

function getVmHost(config) {
  return String(requireConfigValue(config, "host"));
}

function getVmUser(config) {
  return String(config.vm_user ?? "ubuntu");
}

function multipassInfo(vmName) {
  return runCommand("multipass", ["info", vmName]);
}

function vmExists(vmName) {
  return multipassInfo(vmName).status === 0;
}

function parseMultipassIp(infoOutput) {
  const line = infoOutput
    .split(/\r?\n/)
    .find((candidate) => candidate.trim().startsWith("IPv4:"));
  if (!line) return null;

  const ip = line.replace("IPv4:", "").trim().split(/\s+/)[0];
  return ip || null;
}

function getVmIp(vmName) {
  const result = multipassInfo(vmName);
  if (result.status !== 0) {
    throw new Error(`VM not found or unavailable: ${vmName}`);
  }

  const ip = parseMultipassIp(result.stdout);
  if (!ip) throw new Error(`No IPv4 found for VM: ${vmName}`);
  return ip;
}

function runUp(args) {
  ensureTool("multipass");
  const { config } = loadConfig(args);
  const vmName = getVmName(config);

  if (vmExists(vmName)) {
    console.log(`VM already exists: ${vmName}`);
    const info = multipassInfo(vmName);
    if (info.stdout) console.log(info.stdout.trim());
    return;
  }

  const cpusRequested = Number(requireNestedConfigValue(config, "resources", "cpus"));
  const memoryRequested = String(requireNestedConfigValue(config, "resources", "memory"));
  const diskRequested = String(requireNestedConfigValue(config, "resources", "disk"));
  const hostCpuCount = cpus().length;
  const hostFreeMemoryMiB = Math.floor(freemem() / 1024 / 1024);
  const requestedMemoryMiB = parseSizeToMiB(memoryRequested);

  console.log(`VM      ${vmName}`);
  console.log(`image   Ubuntu ${UBUNTU_IMAGE}`);
  console.log(`cpus    ${cpusRequested} requested, ${hostCpuCount} host logical CPUs`);
  console.log(`memory  ${memoryRequested} requested, ${formatMiB(hostFreeMemoryMiB)} currently free`);
  console.log(`disk    ${diskRequested} requested`);

  if (cpusRequested > hostCpuCount) {
    console.log("warning requested CPUs exceed host logical CPUs");
  }
  if (requestedMemoryMiB > hostFreeMemoryMiB) {
    console.log("warning requested memory exceeds currently free memory");
  }

  confirmOrExit(`Create Multipass VM ${vmName}?`, args);

  runCommandChecked(
    "multipass",
    [
      "launch",
      UBUNTU_IMAGE,
      "--name",
      vmName,
      "--cpus",
      String(cpusRequested),
      "--memory",
      memoryRequested,
      "--disk",
      diskRequested
    ],
    { stdio: "inherit" }
  );
}

function runHostsSync(args) {
  ensureTool("multipass");
  const { config } = loadConfig(args);
  const vmName = getVmName(config);
  const vmHost = getVmHost(config);
  const ip = getVmIp(vmName);

  console.log(`${ip} ${vmHost}`);
  confirmOrExit(`Update /etc/hosts for ${vmHost}?`, args);

  const scriptDir = mkdtempSync(join(tmpdir(), "devctl-hosts-"));
  const scriptPath = join(scriptDir, "update-hosts.sh");
  writeFileSync(
    scriptPath,
    `#!/bin/sh
set -eu
ip=${quoteShell(ip)}
host=${quoteShell(vmHost)}
tmp=$(mktemp)
awk -v host="$host" '{ for (i = 2; i <= NF; i++) if ($i == host) next; print }' /etc/hosts > "$tmp"
printf '%s %s\\n' "$ip" "$host" >> "$tmp"
cat "$tmp" > /etc/hosts
rm -f "$tmp"
`,
    { mode: 0o700 }
  );

  try {
    runCommandChecked("sudo", ["sh", scriptPath], { stdio: "inherit" });
  } finally {
    rmSync(scriptDir, { force: true, recursive: true });
  }
}

function runSshConfig(args) {
  const { config } = loadConfig(args);
  const vmName = getVmName(config);
  const vmHost = getVmHost(config);
  const vmUser = getVmUser(config);

  console.log(`Host ${vmName}
  HostName ${vmHost}
  User ${vmUser}
  ForwardAgent yes`);
}

function runProvision(args) {
  ensureTool("ansible-playbook");
  const { config } = loadConfig(args);
  const vmName = getVmName(config);
  const vmHost = getVmHost(config);
  const vmUser = getVmUser(config);
  const playbookPath = resolve(CLI_ROOT, DEFAULT_ANSIBLE_PLAYBOOK);

  if (!existsSync(playbookPath)) {
    throw new Error(`Ansible playbook not found: ${playbookPath}`);
  }

  const inventoryDir = mkdtempSync(join(tmpdir(), "devctl-inventory-"));
  const inventoryPath = join(inventoryDir, "inventory.yml");
  writeFileSync(
    inventoryPath,
    `all:
  children:
    dev_vm:
      hosts:
        ${vmName}:
          ansible_host: ${vmHost}
          ansible_user: ${vmUser}
          ansible_ssh_common_args: "-o ForwardAgent=yes"
`,
    { mode: 0o600 }
  );

  try {
    runCommandChecked(
      "ansible-playbook",
      ["-i", inventoryPath, playbookPath, "--extra-vars", `vm_user=${vmUser}`],
      { stdio: "inherit" }
    );
  } finally {
    rmSync(inventoryDir, { force: true, recursive: true });
  }
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
const subcommand = args._[1];

if (!command || args.help || command === "--help" || command === "-h") {
  printHelp();
  process.exit(0);
}

try {
  if (command === "doctor") runDoctor(args);
  else if (command === "config") runConfig(args);
  else if (command === "init") runInit();
  else if (command === "up") runUp(args);
  else if (command === "hosts" && subcommand === "sync") runHostsSync(args);
  else if (command === "ssh" && subcommand === "config") runSshConfig(args);
  else if (command === "provision") runProvision(args);
  else {
    console.error(`Unknown command: ${args._.join(" ")}`);
    printHelp();
    process.exit(1);
  }
} catch (error) {
  console.error(error instanceof Error ? error.message : String(error));
  process.exit(1);
}
