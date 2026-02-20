// ─── Theme contract ─────────────────────────────────────────────────
// Every built-in theme component receives these props from App.tsx.
// The theme owns ALL rendering — layout, components, styles, animations.
// App.tsx owns ALL logic — state, data fetching, events, callbacks.

export interface Peer {
  Hostname: string;
  DNSName: string;
  TailscaleIP: string;
  Online: boolean;
  OS: string;
  IsSelf: boolean;
  RunningTailchat: boolean;
}

export interface Reaction {
  Emoji: string;
  Sender: string;
}

export interface Message {
  ID: string;
  Sender: string;
  Content: string;
  Timestamp: string;
  IsOwn: boolean;
  GroupID: string;
  State: number; // 0=sending, 1=delivered, 2=read
  Reactions: Reaction[] | null;
}

export interface GIF {
  ID: string;
  Title: string;
  URL: string;
  Media: {
    GIF: { URL: string };
    TinyGIF: { URL: string };
    NanoGIF: { URL: string };
  };
}

export interface ThemeInfo {
  name: string;
  description: string;
  author: string;
  path: string;
  isDefault: boolean;
}

// ─── Group chat types ───────────────────────────────────────────────

export interface Group {
  ID: string;
  Name: string;
  Members: string[];
}

export interface GroupInvite {
  groupID: string;
  groupName: string;
  members: string[];
  from: string;
}

export interface ThemeProps {
  // ─── Data ───────────────────────────────────────────────────
  peers: Peer[];
  selfInfo: { hostname: string; ip: string };
  activePeer: string;          // hostname for DM, or '' if group selected
  activePeerData: Peer | undefined;
  activeGroup: Group | null;   // non-null when a group is selected
  activeChat: string;          // chatKey: hostname for DM, 'group:<id>' for group
  messages: Message[];
  typing: Record<string, boolean>;
  unread: Record<string, number>;
  connected: Record<string, boolean>;
  groups: Group[];
  groupInvites: GroupInvite[];
  themes: ThemeInfo[];
  activeTheme: string;
  error: string;

  // ─── Input state ────────────────────────────────────────────
  inputText: string;
  searchQuery: string;
  showGifPicker: boolean;
  gifQuery: string;
  gifResults: GIF[];
  gifLoading: boolean;
  view: 'chat' | 'settings';

  // ─── Callbacks ──────────────────────────────────────────────
  onSelectPeer: (peer: Peer) => void;
  onSelectGroup: (group: Group) => void;
  onSend: () => void;
  onInputChange: (e: React.ChangeEvent<HTMLTextAreaElement>) => void;
  onInputKeyDown: (e: React.KeyboardEvent) => void;
  onSearchChange: (query: string) => void;
  onReaction: (msgID: string, emoji: string) => void;
  onOpenGifs: () => void;
  onCloseGifs: () => void;
  onSearchGifs: (query: string) => void;
  onPickGif: (gif: GIF) => void;
  onSetView: (view: 'chat' | 'settings') => void;
  onSetTheme: (name: string) => void;
  onCreateGroup: (name: string, members: string[]) => void;
  onAcceptGroupInvite: (invite: GroupInvite) => void;
  onDeclineGroupInvite: (invite: GroupInvite) => void;

  // ─── Refs ───────────────────────────────────────────────────
  messagesEndRef: React.RefObject<HTMLDivElement | null>;
  textareaRef: React.RefObject<HTMLTextAreaElement | null>;
}
