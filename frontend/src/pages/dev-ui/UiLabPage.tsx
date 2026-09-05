import { Clock, LayoutGrid, Library, List, Plus, Rocket, Timer, Trash2 } from "lucide-react";
import { useState, type ReactNode } from "react";

import type { Account } from "../../entities/account/model";
import type { AvailableGameVersion } from "../../entities/game-version/model";
import type { GameVersion } from "../../entities/game-version/model";
import type { Instance } from "../../entities/instance/model";
import type { Operation } from "../../entities/operation/model";
import { AccountCard } from "../../features/accounts/AccountCard";
import { VersionItem } from "../../features/install-game-version/VersionItem";
import { InstanceCard } from "../../features/instances/InstanceCard";
import { ModCard } from "../../features/mods/ModCard";
import { OperationItem } from "../../features/operations/OperationItem";
import { ServerCard } from "../../features/servers/ServerCard";
import { ServerDetailsContent } from "../../features/servers/ServerDetailsDialog";
import { StatCard } from "../../features/statistics/StatCard";
import type { DownloadedMod, ModSummary, FavoriteServer, PublicServer } from "../../shared/api";
import { Button } from "../../shared/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "../../shared/ui/card";
import { CoverArt } from "../../shared/ui/cover-art";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "../../shared/ui/dialog";
import { EmptyState } from "../../shared/ui/empty";
import { ErrorState } from "../../shared/ui/error-state";
import { Field } from "../../shared/ui/field";
import { IconButton } from "../../shared/ui/icon-button";
import { Input } from "../../shared/ui/input";
import { LoadingState } from "../../shared/ui/loading-state";
import { Page, PageContent, PageSection } from "../../shared/ui/page";
import { PageHeader } from "../../shared/ui/page-header";
import { Progress } from "../../shared/ui/progress";
import { SearchInput } from "../../shared/ui/search-input";
import { SectionHeader } from "../../shared/ui/section-header";
import { SegmentedControl } from "../../shared/ui/segmented-control";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../../shared/ui/select";
import { SelectionCheckbox } from "../../shared/ui/selection-checkbox";
import { SettingRow } from "../../shared/ui/setting-row";
import { StatusPill } from "../../shared/ui/status-pill";
import { Stepper } from "../../shared/ui/stepper";
import { Switch } from "../../shared/ui/switch";
import { Tabs } from "../../shared/ui/tabs";
import { Toolbar, ToolbarGroup } from "../../shared/ui/toolbar";
import { TooltipProvider } from "../../shared/ui/tooltip";
import { NavItem } from "../../widgets/layout/NavItem";

const colorGroups = [
  [
    "Surfaces",
    [
      ["Background", "--color-bg-app"],
      ["Surface", "--color-surface-1"],
      ["Raised", "--color-surface-3"],
      ["Border", "--color-border-default"],
    ],
  ],
  [
    "Text",
    [
      ["Primary", "--color-text-primary"],
      ["Muted", "--color-text-muted"],
    ],
  ],
  [
    "States",
    [
      ["Accent", "--color-accent"],
      ["Success", "--color-success"],
      ["Warning", "--color-warning"],
      ["Danger", "--color-danger"],
    ],
  ],
] as const;

const mockVersion: GameVersion = {
  id: "1.20.4",
  name: "Vintage Story 1.20.4",
  channel: "stable",
  platform: "linux",
  architecture: "amd64",
  installationDir: "/mock/game",
  executablePath: "/mock/game/Vintagestory",
  status: "installed",
  sizeBytes: 1,
  installedAt: "2026-01-01T00:00:00Z",
};

const mockAccount: Account = {
  id: "ui-lab-account",
  username: "Waxlighter",
  displayName: "Waxlighter",
  email: "player@example.com",
  status: "valid",
  isDefault: true,
};

const mockAvailableVersion: AvailableGameVersion = {
  id: "1.22.6",
  name: "1.22.6",
  channel: "stable",
  platform: "linux",
  architecture: "amd64",
  downloadSize: 590_500_000,
  latest: true,
  installed: false,
};

const accountCardHandlers = {
  onSelect: () => {},
  onSignInAgain: () => {},
  onValidate: () => {},
  onRemove: () => {},
};

const mockInstance: Instance = {
  id: "ui-lab-instance",
  name: "A Warm Home",
  description: "A quiet survival world with a carefully chosen set of mods.",
  gameVersionId: mockVersion.id,
  gameClient: "vanilla",
  directory: "/mock/instances/a-warm-home",
  status: "ready",
  launchArguments: [],
  environmentVariables: {},
  isPinned: false,
  lastPlayedAt: "2026-08-16T19:30:00Z",
  createdAt: "2026-01-01T00:00:00Z",
  enabledModCount: 12,
  totalModCount: 14,
  playtimeSeconds: 52320,
};

const instanceCardHandlers = {
  onOpen: () => {},
  onEdit: () => {},
  onOpenDirectory: () => {},
  onClone: () => {},
  onExport: () => {},
  onDelete: () => {},
  onLaunch: () => {},
  onStop: async () => {},
  onTogglePin: () => {},
};

const mockMod: ModSummary = {
  id: "ui-lab-mod",
  name: "Player Corpse",
  authorName: "Ada",
  summary: "Creates a recoverable corpse after death, with configurable timers and drop rules.",
  side: "both",
  gameVersions: ["1.20.x", "1.19.x"],
  downloads: 42_000,
  updatedAt: "2026-08-01T10:00:00Z",
  tags: ["Utility"],
  isDownloaded: false,
  isInstalled: false,
  updateAvailable: false,
};

const mockDownloaded: DownloadedMod = {
  modId: "ui-lab-mod",
  name: "Player Corpse",
  authorName: "Ada",
  side: "both",
  versionId: "v7",
  downloadedVersion: "2.0.0",
  gameVersions: ["1.20"],
  fileName: "playercorpse.zip",
  fileSize: 512_000,
  downloadedAt: "2026-08-02T10:00:00Z",
  installedInstances: [],
  updateAvailable: false,
};

const modCardHandlers = {
  onOpen: () => {},
  onInstall: () => {},
  onDelete: () => {},
};

const mockPublicServer: PublicServer = {
  id: "ui-lab-server",
  url: "https://servers.vintagestory.at/s/42",
  name: "The Lighthouse Community",
  address: "lighthouse.example.com:42420",
  description: "A relaxed community server focused on building, trading, and long winters.",
  fullDescription:
    "A relaxed community server focused on building, trading, and long winters. Meet fellow players and make a home by the coast.",
  descriptionHtml:
    "<p>A relaxed community server focused on <strong>building</strong>, trading, and long winters. Meet fellow players and make a home by the coast.</p>",
  imageUrl: "https://placehold.co/640x240/24382b/a3c9a8?text=Lighthouse",
  bannerUrl: "https://placehold.co/960x320/24382b/a3c9a8?text=Lighthouse+Community",
  gameVersion: "1.22.7",
  players: 18,
  maxPlayers: 40,
  modCount: 12,
  location: "Sweden",
  languages: ["English", "Swedish"],
  operator: "Lighthouse Guild",
  operatorUrl: "https://example.com",
  modified: true,
  requiresWhitelist: false,
  accessRestricted: false,
  joinable: true,
  mods: [
    { name: "Better Ruins", version: "0.4.0", url: "https://mods.vintagestory.at" },
    { name: "Carry On", version: "1.14.3", url: "https://mods.vintagestory.at" },
  ],
};

const mockFavorite: FavoriteServer = {
  id: "ui-lab-favorite",
  name: "The Lighthouse Community",
  address: "lighthouse.example.com:42420",
  instanceId: mockInstance.id,
};

const serverCardHandlers = {
  onJoin: () => {},
  onToggleFavorite: () => {},
  onDetails: () => {},
  onCopyAddress: () => {},
  onCopyLink: () => {},
};

const mockOperation: Operation = {
  id: "ui-lab-op",
  type: "mod_download",
  title: "Downloading Better Ruins",
  status: "running",
  progress: 0.63,
  currentBytes: 63_000_000,
  totalBytes: 100_000_000,
  bytesPerSecond: 1_500_000,
  createdAt: "2026-08-17T08:00:00Z",
};

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <PageSection>
      <SectionHeader title={title} />
      {children}
    </PageSection>
  );
}

function LanguageSelectRow() {
  const [language, setLanguage] = useState("en");
  return (
    <Select value={language} onValueChange={setLanguage}>
      <SelectTrigger className="w-[220px]">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="en">English</SelectItem>
        <SelectItem value="sv">Svenska</SelectItem>
      </SelectContent>
    </Select>
  );
}

export function UiLabPage() {
  const [dialogOpen, setDialogOpen] = useState(false);
  const [tab, setTab] = useState<"overview" | "details">("overview");
  const [layout, setLayout] = useState<"grid" | "list">("grid");
  const [toolbarSearch, setToolbarSearch] = useState("");
  const [searchValue, setSearchValue] = useState("");
  const [modSelected, setModSelected] = useState(true);

  return (
    <Page>
      <PageHeader
        eyebrow="Internal reference"
        title="Waxlight UI Lab"
        description="The visual reference for shared Waxlight components and states."
        actions={<Button>Page action</Button>}
      />

      <PageContent className="gap-10 pb-16">
        <Section title="Colors">
          <div className="space-y-5">
            {colorGroups.map(([group, colors]) => (
              <div key={group} className="space-y-2">
                <h3 className="text-xs font-bold tracking-widest text-text-muted uppercase">
                  {group}
                </h3>
                <div className="grid grid-cols-[repeat(auto-fit,minmax(150px,1fr))] gap-3">
                  {colors.map(([label, token]) => (
                    <div
                      key={token}
                      className="overflow-hidden rounded-md border border-border-subtle"
                    >
                      <div className="h-14" style={{ background: `var(${token})` }} />
                      <div className="bg-surface-1 px-3 py-2">
                        <strong className="block text-xs">{label}</strong>
                        <code className="text-[11px] text-text-muted">{token}</code>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </Section>

        <Section title="Typography">
          <Card variant="subtle">
            <CardContent className="space-y-3">
              <h1 className="font-display text-4xl font-semibold">Page heading</h1>
              <h2 className="font-display text-2xl font-semibold">Section heading</h2>
              <p>Normal interface text set in Manrope.</p>
              <p className="text-text-secondary">Secondary supporting text.</p>
              <p className="text-text-muted">Muted metadata and descriptions.</p>
            </CardContent>
          </Card>
        </Section>

        <Section title="Structural patterns">
          <div className="space-y-5">
            <SectionHeader
              variant="compact"
              title="Section heading"
              description="Supporting context for a focused part of the page."
              actions={<Button variant="secondary">Section action</Button>}
            />
            <Toolbar>
              <ToolbarGroup className="min-w-[240px] flex-1">
                <SearchInput
                  wrapperClassName="w-full max-w-sm"
                  aria-label="Toolbar search"
                  placeholder="Search"
                  value={toolbarSearch}
                  onValueChange={setToolbarSearch}
                />
              </ToolbarGroup>
              <ToolbarGroup align="end">
                <Button variant="ghost">Refresh</Button>
                <Button>New item</Button>
              </ToolbarGroup>
            </Toolbar>
            <Tabs
              label="Example views"
              value={tab}
              options={[
                {
                  value: "overview",
                  label: "Overview",
                  tabId: "ui-lab-overview-tab",
                  panelId: "ui-lab-tab-panel",
                },
                {
                  value: "details",
                  label: "Details",
                  tabId: "ui-lab-details-tab",
                  panelId: "ui-lab-tab-panel",
                },
              ]}
              onValueChange={setTab}
            />
            <p
              id="ui-lab-tab-panel"
              className="text-sm text-text-muted"
              role="tabpanel"
              aria-labelledby={tab === "overview" ? "ui-lab-overview-tab" : "ui-lab-details-tab"}
            >
              Selected tab: {tab}
            </p>
            <SegmentedControl
              label="Example layout"
              value={layout}
              options={[
                {
                  value: "grid",
                  label: <LayoutGrid size={16} aria-hidden="true" />,
                  accessibleLabel: "Grid layout",
                },
                {
                  value: "list",
                  label: <List size={16} aria-hidden="true" />,
                  accessibleLabel: "List layout",
                },
              ]}
              onValueChange={setLayout}
            />
            <div className="max-w-60 rounded-md bg-bg-sidebar p-2">
              <NavItem to="/library" icon={Library} label="Navigation item" />
            </div>
          </div>
        </Section>

        <Section title="Buttons">
          <div className="flex flex-wrap items-center gap-3">
            <Button>Primary</Button>
            <Button variant="secondary">Secondary</Button>
            <Button variant="ghost">Ghost</Button>
            <Button variant="danger">Danger</Button>
            <Button disabled>Disabled</Button>
            <Button busy>Loading</Button>
            <Button>
              <Plus size={16} aria-hidden="true" />
              Icon and text
            </Button>
          </div>
        </Section>

        <Section title="Icon buttons">
          <div className="flex items-center gap-3">
            <IconButton aria-label="Add item">
              <Plus size={16} aria-hidden="true" />
            </IconButton>
            <IconButton aria-label="Add item without background" variant="ghost">
              <Plus size={16} aria-hidden="true" />
            </IconButton>
            <IconButton aria-label="Delete item" variant="danger">
              <Trash2 size={16} aria-hidden="true" />
            </IconButton>
            <IconButton aria-label="Disabled action" disabled>
              <Plus size={16} aria-hidden="true" />
            </IconButton>
          </div>
        </Section>

        <Section title="Inputs">
          <div className="grid max-w-3xl gap-4 sm:grid-cols-2">
            <label className="field" htmlFor="ui-lab-default">
              <span>Default</span>
              <Input id="ui-lab-default" defaultValue="Waxlight" />
            </label>
            <label className="field" htmlFor="ui-lab-placeholder">
              <span>Placeholder</span>
              <Input id="ui-lab-placeholder" placeholder="Search instances" />
            </label>
            <label className="field" htmlFor="ui-lab-focus">
              <span>Focus (use Tab)</span>
              <Input id="ui-lab-focus" defaultValue="Focus state" />
            </label>
            <label className="field" htmlFor="ui-lab-disabled">
              <span>Disabled</span>
              <Input id="ui-lab-disabled" disabled value="Unavailable" readOnly />
            </label>
            <label className="field" htmlFor="ui-lab-invalid">
              <span>Invalid</span>
              <Input
                id="ui-lab-invalid"
                aria-invalid="true"
                aria-describedby="ui-lab-input-error"
                defaultValue="Bad value"
              />
              <small id="ui-lab-input-error" className="text-danger">
                Enter a valid value.
              </small>
            </label>
            <Field label="Search input">
              <SearchInput
                aria-label="Search input example"
                placeholder="Search mods"
                value={searchValue}
                onValueChange={setSearchValue}
              />
            </Field>
          </div>
        </Section>

        <Section title="Cover art placeholders">
          <div className="grid grid-cols-[repeat(auto-fill,minmax(min(180px,100%),1fr))] gap-4">
            {["A Warm Home", "Player Corpse", "Better Ruins", "Snowbound", "123", "Терра"].map(
              (name) => (
                <CoverArt key={name} className="aspect-[16/9] rounded-md" seed={name} alt={name} />
              ),
            )}
          </div>
        </Section>

        <Section title="Cards">
          <div className="grid gap-4 lg:grid-cols-3">
            {(["default", "subtle", "elevated"] as const).map((variant) => (
              <Card key={variant} variant={variant}>
                <CardHeader>
                  <CardTitle>{variant[0].toUpperCase() + variant.slice(1)}</CardTitle>
                  <CardDescription>A reusable, domain-neutral surface.</CardDescription>
                </CardHeader>
                <CardContent>
                  Card content preserves Waxlight’s restrained visual language.
                </CardContent>
                <CardFooter>
                  <Button variant="secondary">Action</Button>
                </CardFooter>
              </Card>
            ))}
          </div>
        </Section>

        <Section title="Domain patterns">
          <div className="grid grid-cols-[repeat(auto-fill,minmax(min(300px,100%),1fr))] gap-4">
            <InstanceCard
              instance={mockInstance}
              version={mockVersion}
              updateCount={2}
              {...instanceCardHandlers}
            />
            <InstanceCard
              instance={{
                ...mockInstance,
                id: "ui-lab-long-name",
                name: "A very long translated instance name that must never break card actions",
                description: "",
              }}
              version={mockVersion}
              {...instanceCardHandlers}
            />
            <InstanceCard
              instance={{ ...mockInstance, id: "ui-lab-busy", name: "Starting", status: "ready" }}
              version={mockVersion}
              busy
              {...instanceCardHandlers}
            />
            <InstanceCard
              instance={{
                ...mockInstance,
                id: "ui-lab-error",
                name: "Needs attention",
                status: "failed",
              }}
              version={mockVersion}
              {...instanceCardHandlers}
            />
          </div>
        </Section>

        <Section title="Mod cards">
          <div className="grid grid-cols-[repeat(auto-fill,minmax(min(300px,100%),1fr))] gap-4">
            <ModCard mod={mockMod} layout="grid" {...modCardHandlers} />
            <ModCard
              mod={{
                ...mockMod,
                id: "ui-lab-mod-long-name",
                name: "A very long translated mod name that must never break the card actions row",
                summary: "",
              }}
              layout="grid"
              {...modCardHandlers}
            />
            <ModCard
              mod={{
                ...mockMod,
                id: "ui-lab-mod-long-description",
                summary:
                  "A genuinely long summary that keeps going and going so that the line clamp truncation is clearly visible on the card surface without pushing the actions out of view.",
              }}
              layout="grid"
              {...modCardHandlers}
            />
            <ModCard
              mod={{ ...mockMod, id: "ui-lab-mod-downloaded", isDownloaded: true }}
              downloaded={mockDownloaded}
              layout="grid"
              {...modCardHandlers}
            />
            <ModCard
              mod={{
                ...mockMod,
                id: "ui-lab-mod-installed",
                isDownloaded: true,
                isInstalled: true,
              }}
              downloaded={{
                ...mockDownloaded,
                installedInstances: [
                  {
                    instanceId: "inst-1",
                    instanceName: "A Warm Home",
                    version: "2.0.0",
                    enabled: true,
                  },
                ],
              }}
              layout="grid"
              {...modCardHandlers}
            />
            <ModCard
              mod={{
                ...mockMod,
                id: "ui-lab-mod-update",
                isDownloaded: true,
                updateAvailable: true,
              }}
              downloaded={{ ...mockDownloaded, updateAvailable: true, latestVersion: "2.1.0" }}
              layout="grid"
              {...modCardHandlers}
            />
            <ModCard
              mod={{ ...mockMod, id: "ui-lab-mod-busy" }}
              layout="grid"
              installBusy
              {...modCardHandlers}
            />
            <ModCard
              mod={{ ...mockMod, id: "ui-lab-mod-no-image", imageUrl: undefined }}
              layout="grid"
              {...modCardHandlers}
            />
            <ModCard
              mod={{ ...mockMod, id: "ui-lab-mod-list" }}
              layout="list"
              {...modCardHandlers}
            />
          </div>
        </Section>

        <Section title="Server cards">
          <TooltipProvider>
            <div className="grid grid-cols-[repeat(auto-fill,minmax(min(280px,100%),1fr))] gap-4">
              <ServerCard server={mockPublicServer} {...serverCardHandlers} />
              <ServerCard
                server={mockPublicServer}
                favorite={mockFavorite}
                preferredInstance={mockInstance}
                {...serverCardHandlers}
              />
              <ServerCard
                server={{
                  ...mockPublicServer,
                  name: "Connecting",
                }}
                busy
                {...serverCardHandlers}
              />
              <ServerCard
                server={{
                  ...mockPublicServer,
                  name: "A genuinely long server name that keeps going until it must truncate safely",
                  address: "",
                }}
                {...serverCardHandlers}
              />
              <ServerCard
                server={{
                  ...mockPublicServer,
                  name: "IPv6 address",
                  address: "2001:db8:85a3:8d3:1319:8a2e:370:7348:42420",
                }}
                {...serverCardHandlers}
              />
              <ServerCard
                server={{
                  ...mockPublicServer,
                  players: 128,
                  modCount: 64,
                  requiresWhitelist: true,
                  accessRestricted: true,
                }}
                {...serverCardHandlers}
              />
              <ServerCard
                server={{
                  ...mockPublicServer,
                  description:
                    "A genuinely long server description that keeps going and going so the line clamp truncation is clearly visible on the card surface without pushing the actions out of view.",
                }}
                {...serverCardHandlers}
              />
              <ServerCard
                server={mockPublicServer}
                favorite={{ ...mockFavorite, id: "ui-lab-missing-instance", instanceId: "deleted" }}
                {...serverCardHandlers}
              />
              <ServerCard
                server={{
                  ...mockPublicServer,
                  requiresWhitelist: true,
                  joinable: false,
                }}
                {...serverCardHandlers}
              />
            </div>
          </TooltipProvider>
        </Section>

        <Section title="Server details">
          <div className="max-w-2xl">
            <Card variant="subtle">
              <ServerDetailsContent
                server={mockPublicServer}
                favorite={mockFavorite}
                preferredInstance={mockInstance}
                onToggleFavorite={() => {}}
              />
            </Card>
          </div>
        </Section>

        <Section title="Page states">
          <div className="grid gap-4 lg:grid-cols-3">
            <EmptyState
              title="Nothing here"
              description="A quiet empty state with an action."
              action={<Button>Create</Button>}
            />
            <ErrorState
              title="Could not load"
              description="The request failed without overwhelming the page."
              action={<Button variant="secondary">Retry</Button>}
            />
            <LoadingState>Loading section…</LoadingState>
          </div>
        </Section>

        <Section title="Status elements">
          <div className="flex flex-wrap gap-2">
            <StatusPill status="ready" />
            <StatusPill status="running" />
            <StatusPill status="failed" />
          </div>
          <div className="flex items-center gap-3">
            <SelectionCheckbox
              label="Select mod"
              checked={modSelected}
              onCheckedChange={setModSelected}
            />
            <SelectionCheckbox
              label="Select another mod"
              checked={!modSelected}
              onCheckedChange={(next) => setModSelected(!next)}
            />
            <span className="text-xs text-text-muted">Selection checkbox (card artwork)</span>
          </div>
          <div className="max-w-xl">
            <Progress value={64} aria-label="Example progress" />
          </div>
        </Section>

        <Section title="Progress">
          <div className="max-w-xl space-y-3">
            <Progress value={0} aria-label="Progress at 0%" />
            <Progress value={37} aria-label="Progress at 37%" />
            <Progress value={100} aria-label="Progress complete" />
            <Progress indeterminate aria-label="Indeterminate progress" />
            <Progress compact value={63} aria-label="Compact progress" />
          </div>
        </Section>

        <Section title="Operation items">
          <div className="space-y-3">
            <OperationItem
              operation={{
                ...mockOperation,
                id: "ui-lab-op-queued",
                status: "queued",
                progress: 0,
              }}
              onCancel={() => {}}
            />
            <OperationItem
              operation={{ ...mockOperation, id: "ui-lab-op-running", status: "running" }}
              onCancel={() => {}}
            />
            <OperationItem
              operation={{
                ...mockOperation,
                id: "ui-lab-op-completed",
                status: "completed",
                progress: 1,
              }}
              onRemove={() => {}}
            />
            <OperationItem
              operation={{
                ...mockOperation,
                id: "ui-lab-op-failed",
                status: "failed",
                errorMessage:
                  "The remote server refused the connection and the file could not be downloaded.",
              }}
              onRemove={() => {}}
            />
            <OperationItem
              operation={{ ...mockOperation, id: "ui-lab-op-cancelled", status: "cancelled" }}
              onRemove={() => {}}
            />
            <OperationItem
              operation={{
                ...mockOperation,
                id: "ui-lab-op-long-title",
                title:
                  "A very long translated operation title that must never break the action row or the progress area",
                status: "running",
              }}
              onCancel={() => {}}
            />
            <OperationItem
              operation={{
                ...mockOperation,
                id: "ui-lab-op-long-error",
                status: "failed",
                errorMessage:
                  "A genuinely long error summary that keeps going and going so the line clamp truncation is clearly visible on the operation row without pushing the actions out of view.",
              }}
              onRemove={() => {}}
            />
          </div>
        </Section>

        <Section title="Stat cards">
          <div className="grid grid-cols-[repeat(auto-fill,minmax(min(220px,100%),1fr))] gap-4">
            <StatCard
              icon={Clock}
              label="Total playtime"
              value="124h 32m"
              hint="across all instances"
            />
            <StatCard icon={Rocket} label="Launches" value="1,248" hint="game sessions" />
            <StatCard icon={Timer} label="Average session" value="1h 5m" hint="per launch" />
            <StatCard label="Large number" value="12,482,391" />
            <StatCard label="Zero state" value="0m" />
            <StatCard label="A very long translated statistic label" value="42" />
          </div>
        </Section>

        <Section title="SettingRow">
          <Card variant="subtle" className="max-w-3xl divide-y divide-border-subtle">
            <SettingRow
              title="Confirm deletion"
              description="Ask for confirmation before removing items."
            >
              <Switch label="Confirm deletion" checked onCheckedChange={() => {}} />
            </SettingRow>
            <SettingRow
              title="Language"
              description="The language used across the launcher interface."
            >
              <LanguageSelectRow />
            </SettingRow>
            <SettingRow
              title="Parallel downloads"
              description="Maximum simultaneous downloads for the download queue."
            >
              <Stepper label="Parallel downloads" value={3} min={1} max={10} onChange={() => {}} />
            </SettingRow>
            <SettingRow
              column
              title="Global launch arguments"
              description="Extra command-line arguments passed to every launch."
            >
              <Input
                className="codeInput"
                defaultValue="--debug"
                aria-label="Global launch arguments"
              />
            </SettingRow>
            <SettingRow
              disabled
              title="Disabled setting"
              description="This row cannot be changed right now."
            >
              <Switch
                label="Disabled setting"
                checked={false}
                onCheckedChange={() => {}}
                disabled
              />
            </SettingRow>
            <SettingRow
              title="Storage location"
              description="A very long translated description that keeps going to prove long descriptions wrap cleanly instead of pushing the control off the row."
            >
              <Button variant="secondary">Change…</Button>
            </SettingRow>
            <SettingRow
              title="Automatic backups"
              description="Create a safety backup before applying changes."
              warning="The previous relocation failed. The launcher keeps the current location."
            >
              <Switch label="Automatic backups" checked onCheckedChange={() => {}} />
            </SettingRow>
          </Card>
        </Section>

        <Section title="FormField">
          <div className="grid max-w-3xl gap-4 sm:grid-cols-2">
            <Field label="Display name">
              <Input defaultValue="A Warm Home" />
            </Field>
            <Field label="Version ID" hint="Must match the version folder inside the archive.">
              <Input defaultValue="1.22.6" />
            </Field>
            <Field label="Checksum" error="The SHA-256 checksum does not match the archive.">
              <Input aria-invalid="true" defaultValue="abc123" />
            </Field>
            <Field label="Path" hint="Unavailable right now.">
              <Input disabled value="/data/instances" readOnly />
            </Field>
          </div>
        </Section>

        <Section title="Account cards">
          <div className="grid grid-cols-[repeat(auto-fill,minmax(min(300px,100%),1fr))] gap-4">
            <AccountCard account={mockAccount} {...accountCardHandlers} />
            <AccountCard
              account={{
                ...mockAccount,
                id: "ui-lab-account-2",
                displayName: "Second",
                email: "second@example.com",
                isDefault: false,
              }}
              {...accountCardHandlers}
            />
            <AccountCard
              account={{
                ...mockAccount,
                id: "ui-lab-account-long",
                displayName:
                  "A very long translated display name that must never break card actions",
                isDefault: false,
              }}
              {...accountCardHandlers}
            />
            <AccountCard
              account={{
                ...mockAccount,
                id: "ui-lab-account-expired",
                displayName: "Expired",
                status: "expired",
                isDefault: false,
              }}
              {...accountCardHandlers}
            />
          </div>
        </Section>

        <Section title="Version items">
          <Card variant="subtle" className="max-w-3xl divide-y divide-border-subtle">
            <VersionItem version={mockAvailableVersion} installed={false} onInstall={() => {}} />
            <VersionItem
              version={{ ...mockAvailableVersion, id: "1.22.5", name: "1.22.5", latest: false }}
              installed
              onInstall={() => {}}
            />
            <VersionItem
              version={{
                ...mockAvailableVersion,
                id: "1.23.0-pre.1",
                name: "1.23.0-pre.1",
                channel: "unstable",
                latest: false,
              }}
              installed={false}
              busy
              onInstall={() => {}}
            />
            <VersionItem
              version={{
                ...mockAvailableVersion,
                id: "1.21.0",
                name: "1.21.0",
                channel: "unstable",
                latest: false,
              }}
              installed={false}
              onInstall={() => {}}
            />
            <VersionItem
              version={{
                ...mockAvailableVersion,
                id: "1.22.6-pre.1",
                name: "A very long translated pre-release version name that must never break the row actions",
                channel: "unstable",
                latest: false,
              }}
              installed={false}
              onInstall={() => {}}
            />
          </Card>
        </Section>

        <Section title="Dialog">
          <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
            <DialogTrigger asChild>
              <Button>Open shared dialog</Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <div>
                  <DialogTitle>Shared dialog</DialogTitle>
                  <DialogDescription>
                    This is the real Radix-based Waxlight dialog.
                  </DialogDescription>
                </div>
              </DialogHeader>
              <div className="px-6 py-5 text-text-secondary">
                Keyboard navigation, focus trapping, Escape, and focus return remain provided by
                Radix.
              </div>
              <DialogFooter>
                <DialogClose asChild>
                  <Button variant="secondary">Close dialog</Button>
                </DialogClose>
                <Button onClick={() => setDialogOpen(false)}>Confirm</Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </Section>
      </PageContent>
    </Page>
  );
}
