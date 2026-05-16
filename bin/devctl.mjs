#!/usr/bin/env node
import {
  accessSync,
  constants,
  existsSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  statSync,
  writeFileSync
} from "node:fs";
import { arch, cpus, freemem, platform, release } from "node:os";
import { dirname, join, resolve } from "node:path";
import { tmpdir } from "node:os";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const VERSION = "0.1.0";
const CONFIG_FILE = ".devctl.yml";
const BASE_REQUIRED_TOOLS = ["ansible-playbook", "ssh", "ssh-add", "git", "pnpm"];
const UBUNTU_IMAGE = "24.04";
const LIMA_UBUNTU_IMAGE_TEMPLATE = "template:_images/ubuntu-24.04";
const DEFAULT_ANSIBLE_PLAYBOOK = "infra/ansible/playbooks/dev-vm.yml";
const DEFAULT_APP_DEV_COMMAND = "pnpm dev --host 0.0.0.0";
const DEFAULT_VM_PROVIDER = "auto";
const DEFAULT_SUPABASE_SEED_MODE = "start";
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
  devctl repo sync [--project <path>] [--yes]
  devctl container up [--project <path>]
  devctl container rebuild [--project <path>]
  devctl app dev [--project <path>]
  devctl down [--project <path>] [--supabase-all] [--vm]
  devctl --help

Commands:
  doctor      Check host prerequisites.
  config      Print resolved project config as JSON.
  init        Create a starter .devctl.yml in the current directory.
  up          Create the dev VM if missing.
  hosts sync  Update /etc/hosts for local dev hostnames.
  ssh config  Print the SSH config block for the dev VM.
  provision   Run the Ansible dev VM playbook.
  repo sync   Clone or fast-forward the project repo inside the VM.
  container   Start or rebuild the project devcontainer inside the VM.
  app dev     Run the app dev command in the devcontainer with Infisical secrets.
  down        Stop the project devcontainer and Supabase stack. Pass --vm to stop the VM.
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

function resolveProjectConfigPath(projectPath) {
  const resolved = resolve(projectPath);
  if (existsSync(resolved) && statSync(resolved).isDirectory()) {
    return findConfigPath(resolved);
  }
  return resolved;
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
  const configPath = args.project ? resolveProjectConfigPath(args.project) : findConfigPath();
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

function getConfiguredVmProvider(config) {
  return String(config.vm?.provider ?? config.vm_provider ?? DEFAULT_VM_PROVIDER);
}

function resolveVmProvider(config) {
  const configured = getConfiguredVmProvider(config);
  if (configured !== "auto") return configured;

  const hostPlatform = getHostPlatform();
  if (hostPlatform === "macos" || hostPlatform === "linux" || hostPlatform === "wsl") return "lima";
  if (hostPlatform === "windows") return "wsl2";
  return "unsupported";
}

function requiredToolsForProvider(provider, config = {}) {
  const providerTools = provider === "lima" ? ["limactl"] : [];
  const adapterTools = isInfisicalConfigured(config) ? ["infisical"] : [];
  return [...providerTools, ...BASE_REQUIRED_TOOLS, ...adapterTools];
}

function ensureSupportedProvider(provider) {
  if (provider === "lima") return;
  if (provider === "wsl2") {
    throw new Error("Windows native is not supported yet. Run devctl inside WSL2.");
  }
  throw new Error(`Unsupported dev VM provider: ${provider}`);
}

function ensureProviderTools(provider, config = {}) {
  ensureSupportedProvider(provider);
  for (const tool of requiredToolsForProvider(provider, config)) ensureTool(tool);
}

function installHintsFor(hostPlatform, missing) {
  if (missing.length === 0) return [];

  if (hostPlatform === "macos") {
    return [
      "macOS install hints:",
      missing.includes("limactl") ? "  brew install lima" : null,
      missing.includes("ansible-playbook") ? "  brew install ansible" : null,
      missing.includes("infisical") ? "  brew install infisical/get-cli/infisical" : null,
      missing.includes("pnpm") ? "  brew install pnpm" : null,
      missing.includes("git") ? "  brew install git" : null
    ].filter(Boolean);
  }

  if (hostPlatform === "linux" || hostPlatform === "wsl") {
    return [
      `${hostPlatform === "wsl" ? "WSL2" : "Linux"} install hints:`,
      missing.includes("limactl") ? "  See: https://lima-vm.io/docs/installation/" : null,
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
  const loaded = args.project ? loadConfig(args) : null;
  const provider = resolveVmProvider(loaded?.config ?? {});
  const requiredTools = requiredToolsForProvider(provider, loaded?.config ?? {});

  console.log(`host    ${hostPlatform} ${arch()}`);
  console.log(`provider ${provider}`);

  for (const tool of requiredTools) {
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

function getRepoUrl(config) {
  return String(requireConfigValue(config, "repo"));
}

function getRepoDir(config) {
  return String(requireConfigValue(config, "repo_dir"));
}

function getAppDevCommand(config) {
  return String(config.app?.dev_command ?? DEFAULT_APP_DEV_COMMAND);
}

function getAppPort(config) {
  return String(config.ports?.app ?? "3000");
}

function getInfisicalEnv(config) {
  return String(config.infisical?.default_env ?? "dev");
}

function getInfisicalPath(config) {
  return String(config.infisical?.project_path ?? "/");
}

function getInfisicalProjectId(config) {
  return config.infisical?.project_id ?? config.infisical?.projectId ?? null;
}

function isInfisicalConfigured(config) {
  return config.infisical !== undefined && config.infisical !== null;
}

function isSupabaseEnabled(config) {
  return config.supabase?.enabled === true;
}

function getSupabaseSeedMode(config) {
  return String(config.supabase?.seed ?? DEFAULT_SUPABASE_SEED_MODE);
}

function getLimaVmType(config) {
  const configured = config.vm?.type ?? config.vm_type;
  if (configured) return String(configured);
  return getHostPlatform() === "macos" ? "vz" : "qemu";
}

function getPortMappings(config) {
  return Object.entries(config.ports ?? {})
    .map(([name, rawPort]) => ({ name, port: Number(rawPort) }))
    .filter((entry) => Number.isInteger(entry.port) && entry.port > 0 && entry.port <= 65535);
}

function formatSizeForLima(value) {
  const miB = parseSizeToMiB(value);
  const giB = miB / 1024;
  return Number.isInteger(giB) ? `${giB}GiB` : `${giB.toFixed(2)}GiB`;
}

function quoteYamlString(value) {
  return JSON.stringify(String(value));
}

function renderLimaConfig(config) {
  const vmUser = getVmUser(config);
  const portForwards = getPortMappings(config)
    .map((entry) => `  - guestPort: ${entry.port}
    hostPort: ${entry.port}
    hostIP: "127.0.0.1"`)
    .join("\n");

  return `minimumLimaVersion: 2.0.0
base:
  - ${LIMA_UBUNTU_IMAGE_TEMPLATE}
vmType: ${quoteYamlString(getLimaVmType(config))}
arch: "default"
cpus: ${Number(requireNestedConfigValue(config, "resources", "cpus"))}
memory: ${quoteYamlString(formatSizeForLima(requireNestedConfigValue(config, "resources", "memory")))}
disk: ${quoteYamlString(formatSizeForLima(requireNestedConfigValue(config, "resources", "disk")))}
mounts: []
containerd:
  system: false
  user: false
ssh:
  forwardAgent: true
  loadDotSSHPubKeys: true
user:
  name: ${quoteYamlString(vmUser)}
  home: ${quoteYamlString(`/home/${vmUser}`)}
portForwards:
${portForwards || "  []"}
`;
}

function limaInstanceExists(vmName) {
  const result = runCommand("limactl", ["ls", "--quiet", vmName]);
  if (result.status !== 0) return false;
  return result.stdout.split(/\r?\n/).map((line) => line.trim()).includes(vmName);
}

function limaSshConfigPath(vmName) {
  const result = runCommandChecked("limactl", ["ls", "--format", "{{.SSHConfigFile}}", vmName]);
  const path = result.stdout.trim();
  if (!path) throw new Error(`No Lima SSH config found for VM: ${vmName}`);
  return path;
}

function repoUsesSsh(repoUrl) {
  return repoUrl.startsWith("git@") || repoUrl.startsWith("ssh://");
}

function parseSshRepoTarget(repoUrl) {
  if (repoUrl.startsWith("ssh://")) {
    const parsed = new URL(repoUrl);
    return { host: parsed.hostname, port: parsed.port };
  }

  if (repoUrl.includes("://")) return null;

  const scpStyle = repoUrl.match(/^(?:[^@]+@)?([^:]+):.+$/);
  if (scpStyle) return { host: scpStyle[1], port: "" };

  return null;
}

function requireSshAgentIdentity() {
  ensureTool("ssh-add");
  const result = runCommand("ssh-add", ["-l"]);
  if (result.status === 0) return;

  const stderr = typeof result.stderr === "string" ? result.stderr.trim() : "";
  const stdout = typeof result.stdout === "string" ? result.stdout.trim() : "";
  const detail = stderr || stdout;
  throw new Error(`No SSH agent identity available for forwarded Git access.${detail ? `\n${detail}` : ""}
Run: ssh-add`);
}

function buildSshArgs(config) {
  const provider = resolveVmProvider(config);
  ensureProviderTools(provider, config);
  ensureTool("ssh");

  const vmName = getVmName(config);
  if (provider !== "lima") {
    throw new Error(`SSH is not implemented for provider: ${provider}`);
  }

  const sshConfigPath = limaSshConfigPath(vmName);

  return [
    "-F",
    sshConfigPath,
    "-o",
    "ForwardAgent=yes",
    "-o",
    "ControlMaster=no",
    "-o",
    "StrictHostKeyChecking=accept-new",
    "-o",
    "ServerAliveInterval=30",
    `lima-${vmName}`
  ];
}

function runVmShellChecked(config, script, options = {}) {
  const sshArgs = [...buildSshArgs(config), `bash -lc ${quoteShell(script)}`];
  const result = runCommand("ssh", sshArgs, {
    input: options.input,
    stdio: options.input === undefined ? "inherit" : ["pipe", "inherit", "inherit"]
  });

  if (result.status !== 0) {
    const suffix = result.signal ? ` (signal ${result.signal})` : "";
    throw new Error(`ssh command failed with exit code ${result.status}${suffix}`);
  }
}

function remotePrelude() {
  return `set -euo pipefail
export PATH="/opt/devtools/bin:/usr/local/bin:$PATH"
`;
}

function runUp(args) {
  const { config } = loadConfig(args);
  const provider = resolveVmProvider(config);
  ensureProviderTools(provider, config);
  const vmName = getVmName(config);

  if (provider !== "lima") {
    throw new Error(`up is not implemented for provider: ${provider}`);
  }

  if (limaInstanceExists(vmName)) {
    console.log(`VM already exists: ${vmName}`);
    const info = runCommand("limactl", ["ls", vmName]);
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
  console.log(`provider ${provider}`);
  console.log(`image   Ubuntu ${UBUNTU_IMAGE} via Lima ${getLimaVmType(config)}`);
  console.log(`cpus    ${cpusRequested} requested, ${hostCpuCount} host logical CPUs`);
  console.log(`memory  ${memoryRequested} requested, ${formatMiB(hostFreeMemoryMiB)} currently free`);
  console.log(`disk    ${diskRequested} requested`);

  if (cpusRequested > hostCpuCount) {
    console.log("warning requested CPUs exceed host logical CPUs");
  }
  if (requestedMemoryMiB > hostFreeMemoryMiB) {
    console.log("warning requested memory exceeds currently free memory");
  }

  confirmOrExit(`Create Lima VM ${vmName}?`, args);

  const configDir = mkdtempSync(join(tmpdir(), "devctl-lima-"));
  const configPath = join(configDir, `${vmName}.yaml`);
  writeFileSync(configPath, renderLimaConfig(config), { mode: 0o600 });

  try {
    runCommandChecked("limactl", ["start", "--yes", "--name", vmName, configPath], {
      stdio: "inherit"
    });
  } finally {
    rmSync(configDir, { force: true, recursive: true });
  }
}

function runHostsSync(args) {
  const { config } = loadConfig(args);
  const provider = resolveVmProvider(config);
  ensureSupportedProvider(provider);
  const vmHost = getVmHost(config);
  const hostIp = provider === "lima" ? "127.0.0.1" : null;

  if (!hostIp) throw new Error(`hosts sync is not implemented for provider: ${provider}`);

  console.log(`${hostIp} ${vmHost}`);
  confirmOrExit(`Update /etc/hosts for ${vmHost}?`, args);

  const scriptDir = mkdtempSync(join(tmpdir(), "devctl-hosts-"));
  const scriptPath = join(scriptDir, "update-hosts.sh");
  writeFileSync(
    scriptPath,
    `#!/bin/sh
set -eu
ip=${quoteShell(hostIp)}
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
  const provider = resolveVmProvider(config);
  ensureProviderTools(provider, config);
  const vmName = getVmName(config);

  if (provider !== "lima") {
    throw new Error(`ssh config is not implemented for provider: ${provider}`);
  }

  console.log(readFileSync(limaSshConfigPath(vmName), "utf8").trim());
}

function runProvision(args) {
  const { config } = loadConfig(args);
  const provider = resolveVmProvider(config);
  ensureProviderTools(provider, config);
  const vmName = getVmName(config);
  const vmUser = getVmUser(config);
  const playbookPath = resolve(CLI_ROOT, DEFAULT_ANSIBLE_PLAYBOOK);
  const rolesPath = resolve(CLI_ROOT, "infra/ansible/roles");

  if (provider !== "lima") {
    throw new Error(`provision is not implemented for provider: ${provider}`);
  }
  if (!existsSync(playbookPath)) {
    throw new Error(`Ansible playbook not found: ${playbookPath}`);
  }

  const sshConfigPath = limaSshConfigPath(vmName);
  const inventoryDir = mkdtempSync(join(tmpdir(), "devctl-inventory-"));
  const inventoryPath = join(inventoryDir, "inventory.yml");
  writeFileSync(
    inventoryPath,
    `all:
  children:
    dev_vm:
      hosts:
        ${vmName}:
          ansible_host: ${quoteYamlString(`lima-${vmName}`)}
          ansible_user: ${quoteYamlString(vmUser)}
          ansible_python_interpreter: /usr/bin/python3
          ansible_ssh_common_args: ${quoteYamlString(`-F ${sshConfigPath} -o ForwardAgent=yes -o ControlMaster=no -o StrictHostKeyChecking=accept-new`)}
`,
    { mode: 0o600 }
  );

  try {
    runCommandChecked(
      "ansible-playbook",
      [
        "-i",
        inventoryPath,
        playbookPath,
        "--extra-vars",
        `vm_user=${vmUser}`,
        "--extra-vars",
        `supabase_cli_enabled=${isSupabaseEnabled(config) ? "true" : "false"}`
      ],
      {
        env: {
          ...process.env,
          ANSIBLE_ROLES_PATH: rolesPath
        },
        stdio: "inherit"
      }
    );
  } finally {
    rmSync(inventoryDir, { force: true, recursive: true });
  }
}

function runRepoSync(args) {
  const { config } = loadConfig(args);
  const repoUrl = getRepoUrl(config);
  const repoDir = getRepoDir(config);
  const repoSshTarget = repoUsesSsh(repoUrl) ? parseSshRepoTarget(repoUrl) : null;

  if (repoUsesSsh(repoUrl)) requireSshAgentIdentity();

  console.log(`repo    ${repoUrl}`);
  console.log(`target  ${repoDir}`);
  confirmOrExit(`Sync repo into VM at ${repoDir}?`, args);

  runVmShellChecked(
    config,
    `${remotePrelude()}
repo_url=${quoteShell(repoUrl)}
repo_dir=${quoteShell(repoDir)}
repo_ssh_host=${quoteShell(repoSshTarget?.host ?? "")}
repo_ssh_port=${quoteShell(repoSshTarget?.port ?? "")}
workspace_dir=$(dirname "$repo_dir")
mkdir -p "$workspace_dir"

if [ -n "$repo_ssh_host" ]; then
  mkdir -p "$HOME/.ssh"
  chmod 700 "$HOME/.ssh"
  touch "$HOME/.ssh/known_hosts"
  chmod 600 "$HOME/.ssh/known_hosts"

  known_host_lookup="$repo_ssh_host"
  keyscan_args="-H"
  if [ -n "$repo_ssh_port" ]; then
    known_host_lookup="[$repo_ssh_host]:$repo_ssh_port"
    keyscan_args="$keyscan_args -p $repo_ssh_port"
  fi

  if ! ssh-keygen -F "$known_host_lookup" -f "$HOME/.ssh/known_hosts" >/dev/null; then
    ssh-keyscan $keyscan_args "$repo_ssh_host" >> "$HOME/.ssh/known_hosts"
  fi
fi

if [ -d "$repo_dir/.git" ]; then
  current_url=$(git -C "$repo_dir" remote get-url origin)
  if [ "$current_url" != "$repo_url" ]; then
    echo "Refusing to sync: origin mismatch." >&2
    echo "expected: $repo_url" >&2
    echo "actual:   $current_url" >&2
    exit 1
  fi

  if [ -n "$(git -C "$repo_dir" status --porcelain)" ]; then
    echo "Refusing to sync dirty repo: $repo_dir" >&2
    git -C "$repo_dir" status -sb >&2
    exit 1
  fi

  git -C "$repo_dir" fetch --prune origin
  git -C "$repo_dir" pull --ff-only
else
  if [ -d "$repo_dir" ] && [ -z "$(find "$repo_dir" -mindepth 1 -maxdepth 1 -print -quit)" ]; then
    git clone "$repo_url" "$repo_dir"
  elif [ -e "$repo_dir" ]; then
    echo "Refusing to clone: path exists and is not a Git repo: $repo_dir" >&2
    exit 1
  else
    git clone "$repo_url" "$repo_dir"
  fi
fi

git -C "$repo_dir" status -sb
`
  );
}

function devcontainerConfigCheck(repoDir) {
  return `if [ ! -f "$repo_dir/.devcontainer/devcontainer.json" ] && [ ! -f "$repo_dir/.devcontainer.json" ]; then
  echo "No devcontainer config found in $repo_dir" >&2
  exit 1
fi
`;
}

function runContainer(args, mode) {
  const { config } = loadConfig(args);
  const repoDir = getRepoDir(config);
  const rebuildArgs = mode === "rebuild" ? " --remove-existing-container" : "";

  runVmShellChecked(
    config,
    `${remotePrelude()}
repo_dir=${quoteShell(repoDir)}
if [ ! -d "$repo_dir/.git" ]; then
  echo "Repo missing in VM: $repo_dir" >&2
  echo "Run: devctl repo sync" >&2
  exit 1
fi
${devcontainerConfigCheck(repoDir)}
cd "$repo_dir"
devcontainer up --workspace-folder "$repo_dir"${rebuildArgs}
`
  );
}

function exportInfisicalDotenv(config) {
  ensureTool("infisical");
  const exportArgs = [
    "export",
    "--format=dotenv-export",
    `--env=${getInfisicalEnv(config)}`,
    `--path=${getInfisicalPath(config)}`
  ];
  const projectId = getInfisicalProjectId(config);
  if (projectId) exportArgs.push(`--projectId=${projectId}`);

  const result = runCommand("infisical", exportArgs);
  if (result.status !== 0) {
    const stderr = typeof result.stderr === "string" ? result.stderr.trim() : "";
    const stdout = typeof result.stdout === "string" ? result.stdout.trim() : "";
    throw new Error(`infisical export failed${stderr || stdout ? `:\n${stderr || stdout}` : ""}`);
  }
  return result.stdout;
}

function runAppDev(args) {
  const { config } = loadConfig(args);
  const repoDir = getRepoDir(config);
  const devCommand = getAppDevCommand(config);
  const supabaseEnabled = isSupabaseEnabled(config);
  const supabaseSeedMode = getSupabaseSeedMode(config);
  const dotenvExport = exportInfisicalDotenv(config);

  console.log(`app     http://${getVmHost(config)}:${getAppPort(config)}`);
  console.log(`command ${devCommand}`);
  if (supabaseEnabled) console.log(`supabase seed=${supabaseSeedMode}`);

  runVmShellChecked(
    config,
    `${remotePrelude()}
secrets=$(cat)
repo_dir=${quoteShell(repoDir)}
dev_command=${quoteShell(devCommand)}
supabase_enabled=${quoteShell(supabaseEnabled ? "true" : "false")}
supabase_seed_mode=${quoteShell(supabaseSeedMode)}
if [ ! -d "$repo_dir/.git" ]; then
  echo "Repo missing in VM: $repo_dir" >&2
  echo "Run: devctl repo sync" >&2
  exit 1
fi
${devcontainerConfigCheck(repoDir)}
cd "$repo_dir"

if [ "$supabase_enabled" = "true" ]; then
  if [ ! -f "$repo_dir/supabase/config.toml" ]; then
    echo "Supabase enabled but missing config: $repo_dir/supabase/config.toml" >&2
    exit 1
  fi

  case "$supabase_seed_mode" in
    none|start|reset) ;;
    *)
      echo "Unsupported supabase.seed mode: $supabase_seed_mode" >&2
      echo "Expected: none, start, or reset" >&2
      exit 1
      ;;
  esac

  if [ "$supabase_seed_mode" != "none" ] && [ ! -f "$repo_dir/supabase/seed.sql" ]; then
    echo "warning supabase seed requested but no supabase/seed.sql exists" >&2
  fi

  supabase start --workdir "$repo_dir" --yes

  if [ "$supabase_seed_mode" = "reset" ]; then
    supabase db reset --workdir "$repo_dir" --local --yes
  fi
fi

devcontainer up --workspace-folder "$repo_dir"
printf '%s\\n' "$secrets" | devcontainer exec --workspace-folder "$repo_dir" sh -lc "set -a; . /dev/stdin; set +a; exec $dev_command"
`,
    { input: dotenvExport }
  );
}

function runDown(args) {
  const { config } = loadConfig(args);
  const repoDir = getRepoDir(config);
  const stopVm = args.vm === true;
  const stopAllSupabase = args["supabase-all"] === true;

  runVmShellChecked(
    config,
    `${remotePrelude()}
repo_dir=${quoteShell(repoDir)}
stop_all_supabase=${quoteShell(stopAllSupabase ? "true" : "false")}

if [ -d "$repo_dir" ]; then
  if [ -f "$repo_dir/supabase/config.toml" ]; then
    if [ "$stop_all_supabase" = "true" ]; then
      supabase stop --workdir "$repo_dir" --all
    else
      supabase stop --workdir "$repo_dir"
    fi
  else
    echo "No Supabase config found: $repo_dir/supabase/config.toml"
  fi

  devcontainer_ids=$(docker ps -q --filter "label=devcontainer.local_folder=$repo_dir")
  if [ -n "$devcontainer_ids" ]; then
    docker stop $devcontainer_ids
  else
    echo "No running devcontainer found for $repo_dir"
  fi
else
  echo "Repo directory missing in VM: $repo_dir"
fi
`
  );

  if (!stopVm) return;

  const provider = resolveVmProvider(config);
  ensureProviderTools(provider, config);
  if (provider !== "lima") {
    throw new Error(`VM stop is not implemented for provider: ${provider}`);
  }
  runCommandChecked("limactl", ["stop", getVmName(config)], { stdio: "inherit" });
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
vm:
  provider: auto
  type: vz
supabase:
  enabled: true
  seed: start
infisical:
  project_path: /example
  default_env: dev
app:
  dev_command: pnpm dev --host 0.0.0.0
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
  else if (command === "repo" && subcommand === "sync") runRepoSync(args);
  else if (command === "container" && subcommand === "up") runContainer(args, "up");
  else if (command === "container" && subcommand === "rebuild") runContainer(args, "rebuild");
  else if (command === "app" && subcommand === "dev") runAppDev(args);
  else if (command === "down") runDown(args);
  else {
    console.error(`Unknown command: ${args._.join(" ")}`);
    printHelp();
    process.exit(1);
  }
} catch (error) {
  console.error(error instanceof Error ? error.message : String(error));
  process.exit(1);
}
