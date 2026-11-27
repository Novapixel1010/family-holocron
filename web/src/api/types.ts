// Types mirror the Go JSON models in store/ and server/module/.

export type Role = "admin" | "editor" | "viewer";

export interface UserRef {
  id: number;
  username: string;
  displayName: string;
}

export interface Attribution {
  createdAt: string;
  updatedAt: string;
  createdBy: UserRef | null;
  updatedBy: UserRef | null;
}

export interface User {
  id: number;
  username: string;
  displayName: string;
  email: string;
  role: Role;
  status: "active" | "disabled";
  createdAt: string;
  updatedAt: string;
}

export type Sex = "M" | "F" | "X" | "U";

export interface PersonInput {
  givenName: string;
  surname: string;
  nickname: string;
  sex: Sex;
  birthDate: string;
  birthPlace: string;
  deathDate: string;
  deathPlace: string;
  deceased: boolean;
  about: string;
  portraitMediaId: number | null;
}

export interface Person extends PersonInput, Attribution {
  id: number;
  uid: string;
  birthSort: string;
  deathSort: string;
}

export interface Page<T> {
  items: T[];
  total: number;
}

export type RelationshipType = "parent" | "spouse";

export interface RelationshipInput {
  type: RelationshipType;
  person1Id: number;
  person2Id: number;
  subtype: string;
  startDate: string;
  endDate: string;
  notes: string;
}

export interface Relationship extends RelationshipInput, Attribution {
  id: number;
}

export interface Relative {
  person: Person;
  relationship?: Relationship;
}

export interface Family {
  parents: Relative[];
  spouses: Relative[];
  children: Relative[];
  siblings: Relative[];
}

export interface EventParticipant {
  personId: number;
  role: string;
  name?: string;
}

export interface EventInput {
  type: string;
  title: string;
  date: string;
  place: string;
  description: string;
  relationshipId: number | null;
  participants: EventParticipant[];
}

export interface HolocronEvent extends EventInput, Attribution {
  id: number;
  sortDate: string;
}

export interface TimelineItem extends Attribution {
  key: string;
  kind: "event" | "birth" | "death";
  type: string;
  title: string;
  date: string;
  sortDate: string;
  year: number;
  place: string;
  description: string;
  eventId?: number;
  people: EventParticipant[];
}

export interface TimelineResponse {
  items: TimelineItem[];
  total: number;
  truncated: boolean;
}

export interface Note extends Attribution {
  id: number;
  personId: number;
  title: string;
  body: string;
}

export interface Media extends Attribution {
  id: number;
  uid: string;
  originalName: string;
  mimeType: string;
  sizeBytes: number;
  sha256: string;
  title: string;
  description: string;
  takenDate: string;
  personIds: number[];
}

export interface MediaInput {
  title: string;
  description: string;
  takenDate: string;
  personIds: number[];
}

export interface TreeSummary {
  id: number;
  name: string;
  givenName: string;
  surname: string;
  sex: Sex;
  birthDate: string;
  deathDate: string;
  birthYear: number;
  deathYear: number;
  living: boolean;
  portraitMediaId: number | null;
  hasAbout: boolean;
}

export type GraphRole = "root" | "ancestor" | "descendant" | "partner";

/** A unique person in the flat tree graph. */
export interface GraphPerson extends TreeSummary {
  generation: number;
  role: GraphRole;
  hasMore: boolean;
}

/** A relationship between two people in the tree graph (id = relationship id). */
export interface GraphLink {
  id: number;
  type: RelationshipType;
  source: number; // parent, or first partner
  target: number; // child, or second partner
  subtype: string;
}

export interface TreeNode {
  person: TreeSummary;
  subtype?: string;
  spouses?: TreeSummary[];
  children: TreeNode[];
  hasMore: boolean;
}

export type TreeDirection = "ancestors" | "descendants";

export interface TreeResponse {
  direction: TreeDirection;
  generations: number;
  root: TreeNode;
  count: number;
  people: GraphPerson[];
  links: GraphLink[];
}

export interface FieldChange {
  from: unknown;
  to: unknown;
}

export interface AuditEntry {
  id: number;
  createdAt: string;
  user: UserRef | null;
  username: string;
  action: string;
  entityType: string;
  entityId: number;
  entityLabel: string;
  personIds: number[];
  changes: Record<string, FieldChange> | null;
  summary: string;
}

export interface AuditPage {
  items: AuditEntry[];
  nextBefore: number;
}

export interface Stats {
  persons: number;
  relationships: number;
  events: number;
  notes: number;
  media: number;
  users: number;
  surnames: number;
}

export interface Status {
  version: string;
  schemaVersion: number;
  setupRequired: boolean;
  instanceName: string;
  signedIn: boolean;
}

export interface BackupManifest {
  format: number;
  appVersion: string;
  schemaVersion: number;
  createdAt: string;
  createdBy: string;
  kind: string;
  counts: Stats;
  files: Record<string, { size: number; sha256: string }>;
}

export interface BackupInfo {
  name: string;
  sizeBytes: number;
  createdAt: string;
  kind: "manual" | "safety" | "upload";
  manifest: BackupManifest | null;
}

export interface RestoreResult {
  restored: string;
  safetySnapshot: string;
  counts: Stats;
  sessionKept: boolean;
}

export interface ImportResult {
  persons: number;
  relationships: number;
  events: number;
  notes: number;
  warnings: string[];
}

export interface PluginSlot {
  kind: "nav" | "person.tab" | "settings.section";
  id: string;
  label: string;
  path?: string;
  order: number;
}

export interface PluginInfo {
  id: string;
  name: string;
  version: string;
  description: string;
  slots: PluginSlot[];
  defaultEnabled: boolean;
  enabled: boolean;
  locked: boolean;
}

export interface BioTemplateInput {
  name: string;
  description: string;
  contentMd: string;
  sortOrder: number;
}

export interface BioTemplate extends BioTemplateInput, Attribution {
  id: number;
}
