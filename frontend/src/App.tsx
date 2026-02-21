import { useState, useEffect, useRef, useCallback } from 'react';
import type { Peer, Message, GIF, ThemeInfo, ThemeProps } from './themes/types';
import DefaultTheme from './themes/default/Theme';
import AuroraTheme from './themes/aurora/Theme';
import VaporTheme from './themes/vapor/Theme';
import TerminalTheme from './themes/terminal/Theme';

// ─── Backend bridge ─────────────────────────────────────────────────
const IS_WAILS = typeof (window as any).runtime !== 'undefined';

let _wailsRuntime: any = null;
let _wailsApp: any = null;
let _shim: any = null;

async function loadBackend() {
  if (IS_WAILS) {
    _wailsRuntime = await import('../wailsjs/runtime/runtime');
    _wailsApp = await import('../wailsjs/go/main/App');
  } else {
    _shim = await import('./wails-shim');
  }
}

const backendReady = loadBackend();

function api<T extends (...args: any[]) => any>(name: string): T {
  return ((...args: any[]) => {
    const fn = IS_WAILS ? _wailsApp?.[name] : _shim?.[name];
    if (!fn) return Promise.resolve(undefined);
    return fn(...args);
  }) as unknown as T;
}

const EventsOn = (event: string, fn: (...args: any[]) => void) => {
  if (IS_WAILS) _wailsRuntime?.EventsOn(event, fn);
  else _shim?.EventsOn(event, fn);
};

const GetPeers = api<() => Promise<Peer[]>>('GetPeers');
const GetSelfInfo = api<() => Promise<Record<string, string>>>('GetSelfInfo');
const GetMessages = api<(k: string) => Promise<Message[]>>('GetMessages');
const SendMessage = api<(p: string, c: string) => Promise<void>>('SendMessage');
const SendTyping = api<(k: string, t: boolean) => Promise<void>>('SendTyping');
const SendReaction = api<(k: string, m: string, e: string) => Promise<void>>('SendReaction');
const SendReadReceipts = api<(k: string) => Promise<void>>('SendReadReceipts');
const ConnectToPeer = api<(ip: string) => Promise<void>>('ConnectToPeer');
const IsConnected = api<(h: string) => Promise<boolean>>('IsConnected');
const GetUnread = api<(k: string) => Promise<number>>('GetUnread');
const ClearUnread = api<(k: string) => Promise<void>>('ClearUnread');
const SearchGifs = api<(q: string, l: number) => Promise<GIF[]>>('SearchGifs');
const TrendingGifs = api<(l: number) => Promise<GIF[]>>('TrendingGifs');
const IsReady = api<() => Promise<boolean>>('IsReady');
const NotifyFrontendReady = api<() => Promise<void>>('NotifyFrontendReady');
const _ListThemes = api<() => Promise<ThemeInfo[]>>('ListThemes');
const _GetActiveTheme = api<() => Promise<string>>('GetActiveTheme');
const _SetTheme = api<(n: string) => Promise<void>>('SetTheme');

// ─── Theme registry ─────────────────────────────────────────────────
// Built-in themes are React components. External themes (filesystem) are
// full HTML/CSS/JS replacements served by the Go backend — those require
// a page reload and aren't in this registry.

const THEME_COMPONENTS: Record<string, React.ComponentType<ThemeProps>> = {
  default: DefaultTheme,
  aurora: AuroraTheme,
  vapor: VaporTheme,
  'retro-terminal': TerminalTheme,
};

// ─── App (logic controller) ─────────────────────────────────────────

export default function App() {
  const [peers, setPeers] = useState<Peer[]>([]);
  const [selfInfo, setSelfInfo] = useState({ hostname: '', ip: '' });
  const [activePeer, setActivePeer] = useState('');
  const [messages, setMessages] = useState<Message[]>([]);
  const [inputText, setInputText] = useState('');
  const [searchQuery, setSearchQuery] = useState('');
  const [typing, setTyping] = useState<Record<string, boolean>>({});
  const [unread, setUnread] = useState<Record<string, number>>({});
  const [connected, setConnected] = useState<Record<string, boolean>>({});
  const [showGifPicker, setShowGifPicker] = useState(false);
  const [gifQuery, setGifQuery] = useState('');
  const [gifResults, setGifResults] = useState<GIF[]>([]);
  const [gifLoading, setGifLoading] = useState(false);
  const [ready, setReady] = useState(false);
  const [error, setError] = useState('');
  const [view, setView] = useState<'chat' | 'settings'>('chat');
  const [themes, setThemes] = useState<ThemeInfo[]>([]);
  const [activeTheme, setActiveTheme] = useState('default');

  const messagesEndRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const gifSearchRef = useRef<number | null>(null);
  const typingRef = useRef<number | null>(null);
  const activePeerRef = useRef('');



  useEffect(() => { activePeerRef.current = activePeer; }, [activePeer]);

  // ─── Data fetching ──────────────────────────────────────────────

  const refreshPeers = useCallback(async () => {
    try {
      const p = await GetPeers();
      setPeers(p || []);
      const cs: Record<string, boolean> = {};
      for (const peer of (p || [])) {
        if (!peer.IsSelf) {
          try { cs[peer.Hostname] = await IsConnected(peer.Hostname); } catch { cs[peer.Hostname] = false; }
        }
      }
      setConnected(cs);
    } catch { /* */ }
  }, []);

  const refreshMessages = useCallback(async (key: string) => {
    try { setMessages(await GetMessages(key) || []); } catch { /* */ }
  }, []);

  const refreshUnread = useCallback(async (key: string) => {
    try { GetUnread(key).then(c => setUnread(prev => ({ ...prev, [key]: c }))); } catch { /* */ }
  }, []);

  // ─── Backend events ─────────────────────────────────────────────

  // Shared handler for when the backend is ready — called by either
  // the "ready" event OR the IsReady() poll (whichever fires first).
  const handleReady = useCallback(() => {
    setReady(true);
    GetSelfInfo().then(info => setSelfInfo({ hostname: info?.['hostname'] || '', ip: info?.['ip'] || '' }));
    refreshPeers();
    _GetActiveTheme().then(t => { if (t && THEME_COMPONENTS[t]) setActiveTheme(t); });
  }, [refreshPeers]);

  useEffect(() => {
    backendReady.then(() => {
      EventsOn('ready', handleReady);
      EventsOn('error', (msg: string) => setError(msg));
      EventsOn('peers:updated', (p: Peer[]) => setPeers(p || []));
      EventsOn('chat:message', (d: { chatKey: string }) => {
        if (d.chatKey === activePeerRef.current) refreshMessages(activePeerRef.current);
        refreshUnread(d.chatKey);
      });
      EventsOn('chat:typing', (d: { chatKey: string; isTyping: boolean }) => {
        setTyping(prev => ({ ...prev, [d.chatKey]: d.isTyping }));
      });
      EventsOn('chat:peerConnect', () => refreshPeers());
      EventsOn('chat:reaction', (d: { chatKey: string }) => {
        if (d.chatKey === activePeerRef.current) refreshMessages(activePeerRef.current);
      });
      EventsOn('chat:status', () => refreshPeers());

      NotifyFrontendReady();
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (!ready) return;
    const id = setInterval(() => {
      refreshPeers();
      if (activePeer) refreshMessages(activePeer);
    }, 3000);
    return () => clearInterval(id);
  }, [ready, activePeer]);

  useEffect(() => {
    if (view === 'settings') {
      _ListThemes().then(setThemes);
    }
  }, [view]);

  // ─── Callbacks (passed to theme) ────────────────────────────────

  const onSelectPeer = useCallback(async (peer: Peer) => {
    setActivePeer(peer.Hostname);
    setMessages([]);
    setView('chat');
    if (!connected[peer.Hostname] && peer.RunningTailchat) {
      try { await ConnectToPeer(peer.TailscaleIP); } catch { /* */ }
    }
    await refreshMessages(peer.Hostname);
    ClearUnread(peer.Hostname);
    SendReadReceipts(peer.Hostname);
    setUnread(prev => ({ ...prev, [peer.Hostname]: 0 }));
    setTimeout(() => textareaRef.current?.focus(), 100);
  }, [connected, refreshMessages]);

  const onSend = useCallback(async () => {
    const text = inputText.trim();
    if (!text || !activePeer) return;
    setInputText('');
    try { await SendMessage(activePeer, text); await refreshMessages(activePeer); } catch (e) { setError(String(e)); }
  }, [inputText, activePeer, refreshMessages]);

  const onInputKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); onSend(); }
  }, [onSend]);

  const onInputChange = useCallback((e: React.ChangeEvent<HTMLTextAreaElement>) => {
    setInputText(e.target.value);
    if (activePeer) {
      SendTyping(activePeer, true);
      if (typingRef.current) clearTimeout(typingRef.current);
      typingRef.current = window.setTimeout(() => SendTyping(activePeer, false), 2000);
    }
  }, [activePeer]);

  const onOpenGifs = useCallback(async () => {
    setShowGifPicker(true); setGifQuery(''); setGifLoading(true);
    try { setGifResults(await TrendingGifs(20) || []); } catch { /* */ }
    setGifLoading(false);
  }, []);

  const onCloseGifs = useCallback(() => setShowGifPicker(false), []);

  const onSearchGifs = useCallback((q: string) => {
    setGifQuery(q);
    if (gifSearchRef.current) clearTimeout(gifSearchRef.current);
    if (!q.trim()) { TrendingGifs(20).then(r => setGifResults(r || [])); return; }
    gifSearchRef.current = window.setTimeout(async () => {
      setGifLoading(true);
      try { setGifResults(await SearchGifs(q, 20) || []); } catch { /* */ }
      setGifLoading(false);
    }, 300);
  }, []);

  const onPickGif = useCallback(async (gif: GIF) => {
    setShowGifPicker(false);
    if (!activePeer) return;
    const url = gif.Media.GIF?.URL || gif.URL;
    try { await SendMessage(activePeer, url); await refreshMessages(activePeer); } catch (e) { setError(String(e)); }
  }, [activePeer, refreshMessages]);

  const onReaction = useCallback((msgID: string, emoji: string) => {
    if (!activePeer) return;
    SendReaction(activePeer, msgID, emoji);
    setTimeout(() => refreshMessages(activePeer), 200);
  }, [activePeer, refreshMessages]);

  const onSetTheme = useCallback(async (name: string) => {
    if (THEME_COMPONENTS[name]) {
      // Built-in theme — switch instantly
      setActiveTheme(name);
      await _SetTheme(name);
    } else {
      // External filesystem theme — save and reload
      await _SetTheme(name);
      window.location.reload();
    }
  }, []);

  const onSetView = useCallback((v: 'chat' | 'settings') => setView(v), []);
  const onSearchChange = useCallback((q: string) => setSearchQuery(q), []);

  // Auto-scroll only when new messages arrive (not on poll refreshes).
  // Track previous count so polling the same messages doesn't yank
  // the user back to the bottom while they're reading history.
  const prevMsgCountRef = useRef(0);
  useEffect(() => {
    if (messages.length > prevMsgCountRef.current) {
      messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
    }
    prevMsgCountRef.current = messages.length;
  }, [messages]);

  // ─── Loading state (theme-agnostic) ─────────────────────────────

  if (!ready) {
    return (
      <div className="flex items-center justify-center h-screen w-screen" style={{ background: '#0a0a0f' }}>
        <div className="text-center space-y-4">
          <div className="flex items-center justify-center gap-1.5">
            <span className="w-2 h-2 rounded-full" style={{ background: '#88C0D0', animation: 'pulse-dot 1.2s ease-in-out infinite', animationDelay: '0ms' }} />
            <span className="w-2 h-2 rounded-full" style={{ background: '#88C0D0', animation: 'pulse-dot 1.2s ease-in-out infinite', animationDelay: '200ms' }} />
            <span className="w-2 h-2 rounded-full" style={{ background: '#88C0D0', animation: 'pulse-dot 1.2s ease-in-out infinite', animationDelay: '400ms' }} />
          </div>
          <div>
            <h3 style={{ color: '#e8e8f0', fontSize: '15px', fontWeight: 500 }}>Connecting to Tailscale</h3>
            <p style={{ color: '#4a4a5e', fontSize: '12px', marginTop: '4px' }}>Waiting for the backend to initialize.</p>
          </div>
          {error && <p style={{ color: '#ff6b6b', fontSize: '12px' }}>{error}</p>}
        </div>
      </div>
    );
  }

  // ─── Render active theme ────────────────────────────────────────

  const ThemeComponent = THEME_COMPONENTS[activeTheme] || DefaultTheme;
  const activePeerData = peers.find(p => p.Hostname === activePeer);

  const themeProps: ThemeProps = {
    peers, selfInfo, activePeer, activePeerData, messages, typing, unread, connected,
    themes, activeTheme, error, inputText, searchQuery, showGifPicker, gifQuery,
    gifResults, gifLoading, view,
    onSelectPeer, onSend, onInputChange, onInputKeyDown, onSearchChange,
    onReaction, onOpenGifs, onCloseGifs, onSearchGifs, onPickGif, onSetView, onSetTheme,
    messagesEndRef, textareaRef,
  };

  return <ThemeComponent {...themeProps} />;
}
