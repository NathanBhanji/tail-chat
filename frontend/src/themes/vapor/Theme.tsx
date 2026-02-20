import type { ThemeProps, Message, Peer, ThemeInfo } from '../types';
import './vapor.css';

// ─── Helpers ────────────────────────────────────────────────────────

const isImageURL = (s: string) => {
  const u = s.trim().toLowerCase();
  return u.includes('tenor.com') || u.includes('giphy.com') || /\.(gif|png|jpg|jpeg|webp)(\?.*)?$/.test(u);
};
const formatTime = (ts: string) => new Date(ts).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
const QUICK_REACTIONS = ['👍', '😂', '🔥', '❤️', '✨'];

// ─── Icons ──────────────────────────────────────────────────────────

const IconSearch = () => (
  <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round">
    <circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/>
  </svg>
);

const IconLock = () => (
  <svg width="9" height="9" viewBox="0 0 24 24" fill="currentColor">
    <path d="M18 8h-1V6c0-2.76-2.24-5-5-5S7 3.24 7 6v2H6c-1.1 0-2 .9-2 2v10c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V10c0-1.1-.9-2-2-2zM12 17c-1.1 0-2-.9-2-2s.9-2 2-2 2 .9 2 2-.9 2-2 2zM9 8V6c0-1.66 1.34-3 3-3s3 1.34 3 3v2H9z"/>
  </svg>
);

const IconGear = () => (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
    <circle cx="12" cy="12" r="3"/><path d="M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42"/>
  </svg>
);

const IconX = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round">
    <path d="M18 6L6 18M6 6l12 12"/>
  </svg>
);

const IconSend = () => (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor">
    <path d="M2.01 21L23 12 2.01 3 2 10l15 2-15 2z"/>
  </svg>
);

// ─── Vapor Theme ("Neon Horizon") ───────────────────────────────────
// Layout: Top peer strip → Full-width chat → Bottom input
// NO sidebar. Completely different from default/aurora.

export default function VaporTheme(props: ThemeProps) {
  const {
    peers, selfInfo, activePeer, activePeerData, messages, typing, unread, connected,
    themes, activeTheme, inputText, searchQuery, showGifPicker, gifQuery, gifResults,
    gifLoading, view, onSelectPeer, onSend, onInputChange, onInputKeyDown, onSearchChange,
    onReaction, onOpenGifs, onCloseGifs, onSearchGifs, onPickGif, onSetView, onSetTheme,
    messagesEndRef, textareaRef,
  } = props;

  const filtered = peers.filter(p => !p.IsSelf && p.Hostname.toLowerCase().includes(searchQuery.toLowerCase()));
  const online = filtered.filter(p => p.RunningTailchat);
  const offline = filtered.filter(p => !p.RunningTailchat);
  const allPeers = [...online, ...offline];

  return (
    <div className="vp-root flex flex-col h-screen w-screen">
      {/* Background layers */}
      <div className="vp-grid-bg" />
      <div className="vp-horizon" />

      {/* ── TOP BAR: Brand + Peer strip ── */}
      <div className="vp-topbar relative z-20 shrink-0">
        {/* Drag region */}
        <div className="drag-region h-8" />

        {/* Brand + controls row */}
        <div className="flex items-center justify-between px-5 pb-2">
          <div className="flex items-center gap-3">
            {/* Neon brand mark */}
            <div className="relative">
              <span className="text-[11px] font-black tracking-[0.25em] uppercase" style={{ fontFamily: "'Orbitron', sans-serif", color: '#00ffc8', textShadow: '0 0 12px rgba(0,255,200,0.4)' }}>
                TAILCHAT
              </span>
            </div>
            <span className="text-[10px] text-[rgba(224,214,255,0.2)]">|</span>
            <span className="text-[10px] text-[rgba(224,214,255,0.25)] tracking-wide" style={{ fontFamily: "'Orbitron', sans-serif" }}>
              {selfInfo.hostname}
            </span>
          </div>
          <button
            onClick={() => onSetView(view === 'settings' ? 'chat' : 'settings')}
            className={`p-1.5 rounded-lg transition-all duration-150 cursor-pointer ${view === 'settings' ? 'text-[#00ffc8]' : 'text-[rgba(224,214,255,0.3)] hover:text-[rgba(224,214,255,0.6)]'}`}
          >
            {view === 'settings' ? <IconX /> : <IconGear />}
          </button>
        </div>

        {/* Peer pills — horizontal scroll strip */}
        <div className="px-4 pb-3 overflow-x-auto flex gap-2 items-center">
          <div className="relative shrink-0">
            <div className="absolute left-2 top-1/2 -translate-y-1/2 text-[rgba(224,214,255,0.2)]"><IconSearch /></div>
            <input
              type="text" placeholder="Search..." value={searchQuery}
              onChange={e => onSearchChange(e.target.value)}
              className="vp-input pl-7 pr-2 py-1.5 text-[11px] rounded-full w-[100px] focus:w-[140px] transition-all duration-300"
            />
          </div>
          <div className="w-px h-5 bg-[rgba(255,255,255,0.05)] shrink-0" />
          {allPeers.map(p => (
            <VaporPeerPill
              key={p.Hostname}
              peer={p}
              active={p.Hostname === activePeer}
              unread={unread[p.Hostname] || 0}
              onClick={() => { if (view === 'settings') onSetView('chat'); onSelectPeer(p); }}
            />
          ))}
          {allPeers.length === 0 && <span className="text-[11px] text-[rgba(224,214,255,0.15)] italic">no peers found</span>}
        </div>
      </div>

      {/* ── MAIN CONTENT ── */}
      {view === 'settings' ? (
        <VaporSettings themes={themes} activeTheme={activeTheme} onSetTheme={onSetTheme} />
      ) : !activePeer ? (
        <VaporEmptyState />
      ) : (
        <div className="relative z-10 flex-1 flex flex-col min-h-0">
          {/* Chat header — slim */}
          <div className="flex items-center justify-center gap-3 py-2 shrink-0">
            <span className="text-[13px] font-bold tracking-wide" style={{ fontFamily: "'Orbitron', sans-serif", color: '#e0d6ff' }}>
              {activePeer}
            </span>
            <span className="flex items-center gap-1 text-[9px] text-[#00ffc8] opacity-60" style={{ fontFamily: "'Orbitron', sans-serif" }}>
              <IconLock /> E2E
            </span>
            {connected[activePeer] && (
              <span className="flex items-center gap-1 text-[9px] text-[#00ffc8] opacity-40" style={{ fontFamily: "'Orbitron', sans-serif" }}>
                <span className="w-1.5 h-1.5 rounded-full bg-[#00ffc8] vp-online-dot" /> CONNECTED
              </span>
            )}
            {activePeerData && (
              <span className="text-[9px] text-[rgba(224,214,255,0.2)]" style={{ fontFamily: "'Orbitron', sans-serif" }}>
                {activePeerData.OS}
              </span>
            )}
            {typing[activePeer] && (
              <span className="text-[11px] text-[#ff0080] italic animate-pulse">typing...</span>
            )}
          </div>

          {/* Messages — centered bubbles, iMessage style */}
          <div className="flex-1 overflow-y-auto px-6 pb-3">
            <div className="max-w-[700px] mx-auto">
              {messages.length === 0 && (
                <div className="flex flex-col items-center justify-center h-full text-center pt-20">
                  <div className="w-12 h-12 rounded-xl border border-[rgba(0,255,200,0.1)] flex items-center justify-center mb-3" style={{ boxShadow: '0 0 30px rgba(0,255,200,0.05)' }}>
                    <span className="text-[#00ffc8] opacity-30"><IconLock /></span>
                  </div>
                  <p className="text-[12px] text-[rgba(224,214,255,0.3)]" style={{ fontFamily: "'Orbitron', sans-serif" }}>ENCRYPTED CHANNEL OPEN</p>
                  <p className="text-[11px] text-[rgba(224,214,255,0.12)] mt-1">Send a message to begin</p>
                </div>
              )}
              {messages.map((msg, i) => (
                <VaporBubble key={msg.ID} msg={msg} onReaction={onReaction} prevMsg={i > 0 ? messages[i-1] : null} />
              ))}
              <div ref={messagesEndRef} />
            </div>
          </div>

          {/* Input — full-width neon bar */}
          <div className="relative z-10 shrink-0 px-6 pb-4 pt-2">
            <div className="max-w-[700px] mx-auto flex items-end gap-3">
              <button onClick={onOpenGifs}
                className="shrink-0 text-[10px] font-bold tracking-wider px-3 py-2.5 rounded-xl border border-[rgba(255,255,255,0.06)] text-[rgba(224,214,255,0.3)] hover:text-[#ff0080] hover:border-[rgba(255,0,128,0.2)] transition-all duration-200 cursor-pointer"
                style={{ fontFamily: "'Orbitron', sans-serif" }}>
                GIF
              </button>
              <textarea
                ref={textareaRef}
                placeholder={`Message ${activePeer}...`}
                value={inputText}
                onChange={onInputChange}
                onKeyDown={onInputKeyDown}
                rows={1}
                className="vp-input flex-1 rounded-2xl px-4 py-2.5 text-[13px] min-h-[42px] max-h-32 leading-relaxed resize-none"
              />
              <button
                onClick={onSend}
                disabled={!inputText.trim()}
                className="vp-send-btn shrink-0 px-5 py-2.5 rounded-2xl text-[11px] flex items-center gap-2 cursor-pointer"
              >
                <IconSend /> SEND
              </button>
            </div>
          </div>
        </div>
      )}

      {/* GIF picker — slides up from bottom, centered */}
      {showGifPicker && (
        <div className="fixed inset-0 bg-black/50 backdrop-blur-sm z-50 flex items-end justify-center" onClick={onCloseGifs}>
          <div className="vp-gif-panel w-[520px] max-h-[400px] rounded-t-2xl flex flex-col shadow-2xl" onClick={e => e.stopPropagation()}
            style={{ animation: 'vp-slide-in 0.2s ease-out' }}>
            <div className="p-3 border-b border-[rgba(0,255,200,0.06)] shrink-0">
              <input type="text" placeholder="Search GIFs..." value={gifQuery} onChange={e => onSearchGifs(e.target.value)} autoFocus
                className="vp-input w-full px-4 py-2 text-[12px] rounded-xl" />
            </div>
            <div className="flex-1 overflow-y-auto p-2 grid grid-cols-3 gap-1.5">
              {gifLoading && <p className="col-span-3 text-center py-8 text-[rgba(224,214,255,0.2)] text-[11px]">Loading...</p>}
              {!gifLoading && gifResults.map(gif => (
                <div key={gif.ID} onClick={() => onPickGif(gif)} className="aspect-square rounded-xl overflow-hidden cursor-pointer border border-[rgba(255,255,255,0.04)] hover:border-[rgba(0,255,200,0.15)] hover:scale-[1.03] transition-all duration-150">
                  <img src={gif.Media.TinyGIF?.URL || gif.Media.NanoGIF?.URL || gif.Media.GIF?.URL} alt={gif.Title} loading="lazy" className="w-full h-full object-cover" />
                </div>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// ─── Sub-components ─────────────────────────────────────────────────

function VaporPeerPill({ peer, active, unread, onClick }: { peer: Peer; active: boolean; unread: number; onClick: () => void }) {
  return (
    <button
      onClick={onClick}
      className={`vp-peer-pill shrink-0 flex items-center gap-2 px-3 py-1.5 rounded-full ${active ? 'vp-peer-pill-active' : ''}`}
    >
      {/* Status dot */}
      <span className={`w-2 h-2 rounded-full shrink-0 ${peer.Online ? 'bg-[#00ffc8] vp-online-dot' : 'bg-[rgba(224,214,255,0.15)]'}`} />
      {/* Name */}
      <span className={`text-[11px] font-medium ${active ? 'text-[#00ffc8]' : 'text-[rgba(224,214,255,0.6)]'}`}>
        {peer.Hostname}
      </span>
      {/* Unread badge */}
      {unread > 0 && (
        <span className="min-w-[16px] h-[16px] rounded-full bg-[#ff0080] text-[#0a0a12] text-[9px] font-bold flex items-center justify-center px-1 shadow-[0_0_10px_rgba(255,0,128,0.4)]"
          style={{ fontFamily: "'Orbitron', sans-serif" }}>
          {unread}
        </span>
      )}
    </button>
  );
}

function VaporBubble({ msg, onReaction, prevMsg }: { msg: Message; onReaction: (id: string, emoji: string) => void; prevMsg: Message | null }) {
  const words = msg.Content.trim().split(/\s+/);
  const isGif = words.length === 1 && isImageURL(words[0]);
  const reactions = new Map<string, string[]>();
  if (msg.Reactions) for (const r of msg.Reactions) reactions.set(r.Emoji, [...(reactions.get(r.Emoji) || []), r.Sender]);

  // Show timestamp if sender changed or >5min gap
  const showMeta = !prevMsg || prevMsg.Sender !== msg.Sender;

  return (
    <div className={`vp-msg-enter group/msg mb-1 ${showMeta ? 'mt-4' : 'mt-0.5'}`}>
      {/* Sender + time label */}
      {showMeta && (
        <div className={`flex items-center gap-2 mb-1.5 ${msg.IsOwn ? 'justify-end' : 'justify-start'}`}>
          <span className={`text-[10px] font-bold uppercase tracking-wider ${msg.IsOwn ? 'text-[#00ffc8]' : 'text-[#ff0080]'}`}
            style={{ fontFamily: "'Orbitron', sans-serif", opacity: 0.5 }}>
            {msg.IsOwn ? 'YOU' : msg.Sender}
          </span>
          <span className="text-[9px] text-[rgba(224,214,255,0.15)]">{formatTime(msg.Timestamp)}</span>
        </div>
      )}

      {/* Bubble — right-aligned for own, left for theirs */}
      <div className={`relative flex ${msg.IsOwn ? 'justify-end' : 'justify-start'}`}>
        <div className={`${msg.IsOwn ? 'vp-bubble-own' : 'vp-bubble-them'} px-4 py-2.5 relative`}>
          {isGif ? (
            <div className="rounded-xl overflow-hidden max-w-[260px] -m-1">
              <img src={words[0]} alt="GIF" className="block w-full h-auto" />
            </div>
          ) : (
            <p className="text-[13px] leading-[1.55] break-words text-[#e0d6ff]">{msg.Content}</p>
          )}

          {/* Read/Delivered status */}
          {msg.IsOwn && (
            <span className="block text-right text-[9px] mt-1 opacity-40" style={{ fontFamily: "'Orbitron', sans-serif", color: msg.State === 2 ? '#00ffc8' : '#e0d6ff' }}>
              {msg.State === 2 ? 'READ' : msg.State === 1 ? 'DELIVERED' : 'SENDING'}
            </span>
          )}

          {/* Reactions */}
          {reactions.size > 0 && (
            <div className={`flex gap-1 mt-1.5 flex-wrap ${msg.IsOwn ? 'justify-end' : 'justify-start'}`}>
              {Array.from(reactions.entries()).map(([emoji, senders]) => (
                <button key={emoji} onClick={() => onReaction(msg.ID, emoji)} title={senders.join(', ')}
                  className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] border border-[rgba(255,255,255,0.06)] bg-[rgba(255,255,255,0.03)] hover:border-[rgba(0,255,200,0.15)] transition-colors duration-100 cursor-pointer">
                  {emoji} <span className="text-[9px] text-[rgba(224,214,255,0.3)]" style={{ fontFamily: "'Orbitron', sans-serif" }}>{senders.length}</span>
                </button>
              ))}
            </div>
          )}
        </div>

        {/* Hover reaction bar */}
        <div className={`vp-reactions-bar absolute ${msg.IsOwn ? 'right-0 -top-7' : 'left-0 -top-7'} flex items-center gap-0.5 bg-[rgba(15,15,25,0.92)] border border-[rgba(0,255,200,0.08)] rounded-lg shadow-lg px-0.5 py-0.5 backdrop-blur-sm`}>
          {QUICK_REACTIONS.map(emoji => (
            <button key={emoji} onClick={() => onReaction(msg.ID, emoji)}
              className="w-6 h-6 flex items-center justify-center rounded text-[12px] hover:bg-[rgba(0,255,200,0.06)] transition-colors cursor-pointer">
              {emoji}
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}

function VaporEmptyState() {
  return (
    <div className="relative z-10 flex-1 flex flex-col items-center justify-center text-center px-12" style={{ animation: 'vp-fade-in 0.3s ease-out' }}>
      {/* Neon diamond icon */}
      <div className="relative mb-8">
        <div className="w-20 h-20 rotate-45 border border-[rgba(0,255,200,0.15)] flex items-center justify-center"
          style={{ boxShadow: '0 0 40px rgba(0,255,200,0.06), inset 0 0 20px rgba(0,255,200,0.02)' }}>
          <span className="-rotate-45 text-[#00ffc8] opacity-25">
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
              <path d="M12 2L3 7v5c0 5.55 3.84 10.74 9 12 5.16-1.26 9-6.45 9-12V7l-9-5z"/>
            </svg>
          </span>
        </div>
        {/* Corner accents */}
        <div className="absolute -top-1 -left-1 w-3 h-3 border-t border-l border-[rgba(0,255,200,0.2)]" />
        <div className="absolute -top-1 -right-1 w-3 h-3 border-t border-r border-[rgba(0,255,200,0.2)]" />
        <div className="absolute -bottom-1 -left-1 w-3 h-3 border-b border-l border-[rgba(0,255,200,0.2)]" />
        <div className="absolute -bottom-1 -right-1 w-3 h-3 border-b border-r border-[rgba(0,255,200,0.2)]" />
      </div>
      <h3 className="text-[14px] font-bold tracking-[0.2em] uppercase text-[#e0d6ff] mb-2" style={{ fontFamily: "'Orbitron', sans-serif" }}>
        SELECT A PEER
      </h3>
      <p className="text-[11px] text-[rgba(224,214,255,0.2)] max-w-[300px] leading-relaxed">
        X25519 key exchange / XChaCha20-Poly1305 encryption
      </p>
      <div className="mt-6 flex items-center gap-3">
        <span className="w-12 h-px bg-gradient-to-r from-transparent to-[rgba(0,255,200,0.1)]" />
        <span className="text-[9px] tracking-[0.3em] uppercase text-[#ff0080] opacity-30" style={{ fontFamily: "'Orbitron', sans-serif" }}>NEON HORIZON</span>
        <span className="w-12 h-px bg-gradient-to-l from-transparent to-[rgba(0,255,200,0.1)]" />
      </div>
    </div>
  );
}

function VaporSettings({ themes, activeTheme, onSetTheme }: { themes: ThemeInfo[]; activeTheme: string; onSetTheme: (name: string) => void }) {
  return (
    <div className="vp-settings-overlay relative z-10 flex-1 flex flex-col" style={{ animation: 'vp-fade-in 0.15s ease-out' }}>
      <div className="flex-1 overflow-y-auto flex items-start justify-center py-10 px-6">
        <div className="w-full max-w-[500px]">
          <h2 className="text-[16px] font-bold tracking-[0.2em] uppercase text-[#e0d6ff] mb-1" style={{ fontFamily: "'Orbitron', sans-serif" }}>
            SETTINGS
          </h2>
          <p className="text-[11px] text-[rgba(224,214,255,0.2)] mb-8">Configuration & themes</p>

          <h3 className="text-[11px] font-bold tracking-[0.15em] uppercase text-[#00ffc8] mb-3 opacity-60" style={{ fontFamily: "'Orbitron', sans-serif" }}>THEMES</h3>
          <p className="text-[11px] text-[rgba(224,214,255,0.25)] mb-4 leading-relaxed">
            Choose a theme or drop custom ones into <code className="text-[#00ffc8] opacity-60 bg-[rgba(0,255,200,0.04)] px-1.5 py-0.5 rounded border border-[rgba(0,255,200,0.08)]">~/.tailchat/themes/</code>
          </p>

          <div className="space-y-2 mb-10">
            {themes.map(t => (
              <div key={t.name} onClick={() => onSetTheme(t.name)}
                className={`vp-theme-card p-4 rounded-xl flex items-center gap-4 ${t.name === activeTheme ? 'vp-theme-card-active' : ''}`}>
                <div className="w-8 h-8 rounded-lg border border-[rgba(255,255,255,0.06)] flex items-center justify-center text-[14px] bg-[rgba(255,255,255,0.02)]">
                  {t.name === 'vapor' ? '🌊' : t.name === 'aurora' ? '❄️' : t.name === 'default' ? '🌑' : '🎨'}
                </div>
                <div className="flex-1 min-w-0">
                  <div className="text-[12px] font-bold tracking-wider uppercase" style={{ fontFamily: "'Orbitron', sans-serif", color: t.name === activeTheme ? '#00ffc8' : '#e0d6ff' }}>{t.name}</div>
                  {t.description && <div className="text-[10px] text-[rgba(224,214,255,0.25)] mt-0.5">{t.description}</div>}
                  {t.author && <div className="text-[9px] text-[rgba(224,214,255,0.12)] mt-0.5 uppercase tracking-wider" style={{ fontFamily: "'Orbitron', sans-serif" }}>by {t.author}</div>}
                </div>
                {t.name === activeTheme && (
                  <span className="text-[9px] font-bold tracking-[0.2em] uppercase text-[#00ffc8] shrink-0" style={{ fontFamily: "'Orbitron', sans-serif" }}>ACTIVE</span>
                )}
              </div>
            ))}
          </div>

          <h3 className="text-[11px] font-bold tracking-[0.15em] uppercase text-[#ff0080] mb-3 opacity-40" style={{ fontFamily: "'Orbitron', sans-serif" }}>CREATE A THEME</h3>
          <div className="border border-[rgba(255,255,255,0.04)] rounded-xl p-5 text-[11px] text-[rgba(224,214,255,0.25)] leading-[1.7] space-y-2">
            <p>Create a folder in <code className="text-[#00ffc8] opacity-60 bg-[rgba(0,255,200,0.04)] px-1 py-0.5 rounded">~/.tailchat/themes/my-theme/</code> with an <code className="text-[#00ffc8] opacity-60 bg-[rgba(0,255,200,0.04)] px-1 py-0.5 rounded">index.html</code>.</p>
            <p>Your theme has full access to the Go backend via <code className="text-[#00ffc8] opacity-60 bg-[rgba(0,255,200,0.04)] px-1 py-0.5 rounded">window.go.main.App.*</code> and <code className="text-[#00ffc8] opacity-60 bg-[rgba(0,255,200,0.04)] px-1 py-0.5 rounded">window.runtime.*</code>.</p>
          </div>
        </div>
      </div>
    </div>
  );
}
