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

export interface ThemeProps {
  // ─── Data ───────────────────────────────────────────────────
  peers: Peer[];
  selfInfo: { hostname: string; ip: string };
  activePeer: string;
  activePeerData: Peer | undefined;
  messages: Message[];
  typing: Record<string, boolean>;
  unread: Record<string, number>;
  connected: Record<string, boolean>;
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

  // ─── Refs ───────────────────────────────────────────────────
  messagesEndRef: React.RefObject<HTMLDivElement | null>;
  textareaRef: React.RefObject<HTMLTextAreaElement | null>;
}
