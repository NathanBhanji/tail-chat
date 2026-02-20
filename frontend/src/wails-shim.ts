// wails-shim.ts — Provides mock implementations of Wails runtime and Go
// bindings when running in a regular browser (not inside a Wails webview).
// This allows previewing and iterating on the UI without running the Go backend.

const IS_WAILS = typeof (window as any).runtime !== 'undefined';

// ─── Mock data ──────────────────────────────────────────────────────

const MOCK_PEERS = [
  { Hostname: 'alice-macbook', DNSName: 'alice-macbook.tail1234.ts.net', TailscaleIP: '100.64.0.2', Online: true, OS: 'macOS', IsSelf: false, RunningTailchat: true },
  { Hostname: 'bob-desktop', DNSName: 'bob-desktop.tail1234.ts.net', TailscaleIP: '100.64.0.3', Online: true, OS: 'linux', IsSelf: false, RunningTailchat: true },
  { Hostname: 'charlie-laptop', DNSName: 'charlie-laptop.tail1234.ts.net', TailscaleIP: '100.64.0.4', Online: true, OS: 'windows', IsSelf: false, RunningTailchat: false },
  { Hostname: 'dave-server', DNSName: 'dave-server.tail1234.ts.net', TailscaleIP: '100.64.0.5', Online: false, OS: 'linux', IsSelf: false, RunningTailchat: false },
  { Hostname: 'eve-phone', DNSName: 'eve-phone.tail1234.ts.net', TailscaleIP: '100.64.0.6', Online: false, OS: 'iOS', IsSelf: false, RunningTailchat: false },
];

const MOCK_MESSAGES: Record<string, any[]> = {
  'alice-macbook': [
    { ID: '1', Sender: 'me', Content: 'hey alice, check out this encrypted chat', Timestamp: new Date(Date.now() - 300000).toISOString(), IsOwn: true, State: 2, Reactions: [{ Emoji: '🔥', Sender: 'alice-macbook' }] },
    { ID: '2', Sender: 'alice-macbook', Content: 'this is so cool! fully e2e encrypted over tailscale?', Timestamp: new Date(Date.now() - 240000).toISOString(), IsOwn: false, State: 0, Reactions: null },
    { ID: '3', Sender: 'me', Content: 'yep, X25519 + XChaCha20-Poly1305. zero trust architecture.', Timestamp: new Date(Date.now() - 180000).toISOString(), IsOwn: true, State: 2, Reactions: null },
    { ID: '4', Sender: 'alice-macbook', Content: 'https://media.tenor.com/Km11GYbvYY0AAAAC/good-morning.gif', Timestamp: new Date(Date.now() - 120000).toISOString(), IsOwn: false, State: 0, Reactions: [{ Emoji: '😂', Sender: 'me' }, { Emoji: '😂', Sender: 'alice-macbook' }] },
    { ID: '5', Sender: 'me', Content: 'lol nice gif, they render natively now — no more terminal hacks', Timestamp: new Date(Date.now() - 60000).toISOString(), IsOwn: true, State: 1, Reactions: null },
    { ID: '6', Sender: 'alice-macbook', Content: 'the theming system is great too. i want to make a retro terminal theme', Timestamp: new Date(Date.now() - 30000).toISOString(), IsOwn: false, State: 0, Reactions: null },
  ],
  'bob-desktop': [
    { ID: '10', Sender: 'bob-desktop', Content: 'yo, just set up tailchat on my linux box', Timestamp: new Date(Date.now() - 600000).toISOString(), IsOwn: false, State: 0, Reactions: null },
    { ID: '11', Sender: 'me', Content: 'nice! try sending a gif', Timestamp: new Date(Date.now() - 500000).toISOString(), IsOwn: true, State: 2, Reactions: null },
  ],
};

const MOCK_GIFS = Array.from({ length: 12 }, (_, i) => ({
  ID: `gif-${i}`,
  Title: `GIF ${i}`,
  URL: `https://media.tenor.com/example${i}.gif`,
  Media: {
    GIF: { URL: `https://media.tenor.com/images/example${i}/tenor.gif` },
    TinyGIF: { URL: `https://media.tenor.com/images/example${i}/tenor.gif` },
    NanoGIF: { URL: `https://media.tenor.com/images/example${i}/tenor.gif` },
  },
}));

// For mock gifs, use placeholder images
const PLACEHOLDER_GIFS = Array.from({ length: 12 }, (_, i) => ({
  ID: `gif-${i}`,
  Title: ['happy dance', 'thumbs up', 'mind blown', 'deal with it', 'mic drop',
    'facepalm', 'slow clap', 'eye roll', 'party time', 'high five',
    'confused', 'celebration'][i],
  URL: '',
  Media: {
    GIF: { URL: `https://picsum.photos/200/200?random=${i + 10}` },
    TinyGIF: { URL: `https://picsum.photos/150/150?random=${i + 10}` },
    NanoGIF: { URL: `https://picsum.photos/100/100?random=${i + 10}` },
  },
}));

// ─── Event system stub ──────────────────────────────────────────────

type Listener = (...args: any[]) => void;
const listeners: Record<string, Listener[]> = {};

export function EventsOn(event: string, fn: Listener) {
  if (IS_WAILS) {
    return (window as any).runtime.EventsOn(event, fn);
  }
  if (!listeners[event]) listeners[event] = [];
  listeners[event].push(fn);
}

function emit(event: string, ...args: any[]) {
  for (const fn of (listeners[event] || [])) fn(...args);
}

// Fire ready after a delay — long enough for React to mount and register listeners
if (!IS_WAILS) {
  setTimeout(() => emit('ready', true), 800);
}

// ─── Go binding stubs ───────────────────────────────────────────────

export async function GetPeers() { return MOCK_PEERS; }
export async function GetSelfInfo() { return { hostname: 'my-macbook', ip: '100.64.0.1' }; }
export async function GetMessages(chatKey: string) { return MOCK_MESSAGES[chatKey] || []; }
export async function SendMessage(peer: string, content: string) {
  if (!MOCK_MESSAGES[peer]) MOCK_MESSAGES[peer] = [];
  MOCK_MESSAGES[peer].push({
    ID: `msg-${Date.now()}`,
    Sender: 'me',
    Content: content,
    Timestamp: new Date().toISOString(),
    IsOwn: true,
    State: 1,
    Reactions: null,
  });
}
export async function SendTyping(_key: string, _typing: boolean) {}
export async function SendReaction(_key: string, _msgID: string, _emoji: string) {}
export async function SendReadReceipts(_key: string) {}
export async function ConnectToPeer(_ip: string) {}
export async function IsConnected(_hostname: string) { return true; }
export async function GetUnread(key: string) { return key === 'bob-desktop' ? 2 : 0; }
export async function ClearUnread(_key: string) {}
export async function GetPeerStatus(_hostname: string) { return 'available'; }
export async function SearchGifs(_query: string, _limit: number) { return PLACEHOLDER_GIFS; }
export async function TrendingGifs(_limit: number) { return PLACEHOLDER_GIFS; }
export async function SearchMessages(_query: string) { return {}; }
export async function SendGroupMessage(_groupID: string, _content: string) {}
export async function CreateGroup(_name: string, _members: string[]) { return { ID: '', Name: '', Members: [] }; }
export async function GetGroups() { return []; }
export async function SetStatus(_state: string) {}
export async function ListThemes() {
  return [
    { name: 'default', description: 'Built-in default theme', author: 'tailchat', path: '', isDefault: true },
    { name: 'aurora', description: 'Nord frost with frosted glass panels', author: 'tailchat', path: '', isDefault: false },
    { name: 'vapor', description: 'Neon synthwave with iMessage-style bubbles', author: 'tailchat', path: '', isDefault: false },
    { name: 'retro-terminal', description: 'Green phosphor CRT aesthetic', author: 'tailchat', path: '', isDefault: false },
  ];
}
export async function GetActiveTheme() { return 'default'; }
export async function SetTheme(_name: string) {}
