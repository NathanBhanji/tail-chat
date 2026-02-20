import type { ThemeProps, Message, Peer, ThemeInfo } from '../types';
import './aurora.css';

// ─── Helpers ────────────────────────────────────────────────────────

const isImageURL = (s: string) => {
  const u = s.trim().toLowerCase();
  return u.includes('tenor.com') || u.includes('giphy.com') || /\.(gif|png|jpg|jpeg|webp)(\?.*)?$/.test(u);
};
const formatTime = (ts: string) => new Date(ts).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
const initials = (h: string) => h.slice(0, 2).toUpperCase();

const QUICK_REACTIONS = ['👍', '😂', '🔥', '❤️', '✨'];

// ─── Icons ──────────────────────────────────────────────────────────

const IconSearch = () => (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
    <circle cx="11" cy="11" r="8"/><path d="m21 21-4.35-4.35"/>
  </svg>
);

const IconShield = () => (
  <svg width="11" height="11" viewBox="0 0 24 24" fill="currentColor" opacity="0.6">
    <path d="M12 2L3 7v5c0 5.55 3.84 10.74 9 12 5.16-1.26 9-6.45 9-12V7l-9-5z"/>
  </svg>
);

const IconSettings = () => (
  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
    <circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>
  </svg>
);

const IconSend = () => (
  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M5 12h14M12 5l7 7-7 7"/>
  </svg>
);

const IconLock = () => (
  <svg width="10" height="10" viewBox="0 0 24 24" fill="currentColor">
    <path d="M18 8h-1V6c0-2.76-2.24-5-5-5S7 3.24 7 6v2H6c-1.1 0-2 .9-2 2v10c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V10c0-1.1-.9-2-2-2zM12 17c-1.1 0-2-.9-2-2s.9-2 2-2 2 .9 2 2-.9 2-2 2zM9 8V6c0-1.66 1.34-3 3-3s3 1.34 3 3v2H9z"/>
  </svg>
);

const IconSnowflake = () => (
  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
    <path d="M12 2v20M17 7l-5-5-5 5M17 17l-5 5-5-5M2 12h20M7 7L2 12l5 5M17 7l5 5-5 5"/>
  </svg>
);

// ─── Message grouping ───────────────────────────────────────────────

interface MsgGroup { sender: string; isOwn: boolean; time: string; messages: Message[]; }

function groupMessages(msgs: Message[]): MsgGroup[] {
  const groups: MsgGroup[] = [];
  let cur: MsgGroup | null = null;
  for (const m of msgs) {
    if (!cur || cur.sender !== m.Sender) {
      cur = { sender: m.Sender, isOwn: m.IsOwn, time: formatTime(m.Timestamp), messages: [m] };
      groups.push(cur);
    } else {
      cur.messages.push(m);
    }
  }
  return groups;
}

// ─── Aurora Theme ("Nord Frost") ────────────────────────────────────

export default function AuroraTheme(props: ThemeProps) {
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

  return (
    <div className="au-root flex h-screen w-screen">
      {/* Aurora borealis ambient background */}
      <div className="au-ambient" />

      {/* ── Sidebar — frosted glass ── */}
      <aside className="au-frost relative z-10 w-[280px] min-w-[280px] flex flex-col h-full border-r border-white/[0.06]">
        {/* Brand */}
        <div className="drag-region pt-11 px-5 pb-3 shrink-0">
          <div className="flex items-center gap-2">
            <span className="au-icon-frost"><IconSnowflake /></span>
            <h1 className="text-[16px] font-semibold tracking-tight text-[#ECEFF4]">aurora</h1>
          </div>
          <p className="text-[11px] text-[#5D6882] mt-1" style={{ fontFamily: "'IBM Plex Mono', monospace" }}>
            {selfInfo.hostname} <span className="text-[#88C0D0] opacity-40">&middot;</span> {selfInfo.ip}
          </p>
        </div>

        {/* Search */}
        <div className="px-4 py-2 shrink-0">
          <div className="relative">
            <div className="absolute left-2.5 top-1/2 -translate-y-1/2 text-[#5D6882]"><IconSearch /></div>
            <input type="text" placeholder="Find a peer..." value={searchQuery}
              onChange={e => onSearchChange(e.target.value)}
              className="au-input-frost w-full pl-8 pr-3 py-[8px] text-[13px] rounded-xl" />
          </div>
        </div>

        {/* Peers */}
        <div className="flex-1 overflow-y-auto px-3 py-1">
          {online.length > 0 && (
            <>
              <div className="text-[10px] font-medium uppercase tracking-[0.12em] text-[#5D6882] px-2 pt-4 pb-2" style={{ fontFamily: "'IBM Plex Mono', monospace" }}>Online &mdash; {online.length}</div>
              {online.map(p => <AuroraPeerItem key={p.Hostname} peer={p} active={p.Hostname === activePeer} unread={unread[p.Hostname] || 0} connected={connected[p.Hostname] || false} onClick={() => onSelectPeer(p)} />)}
            </>
          )}
          {offline.length > 0 && (
            <>
              <div className="text-[10px] font-medium uppercase tracking-[0.12em] text-[#5D6882] px-2 pt-4 pb-2" style={{ fontFamily: "'IBM Plex Mono', monospace" }}>Offline &mdash; {offline.length}</div>
              {offline.map(p => <AuroraPeerItem key={p.Hostname} peer={p} active={p.Hostname === activePeer} unread={0} connected={false} onClick={() => onSelectPeer(p)} />)}
            </>
          )}
          {filtered.length === 0 && <p className="px-2 py-6 text-[#5D6882] text-[12px] text-center">No peers found.</p>}
        </div>

        {/* Settings */}
        <div className="px-3 py-2.5 border-t border-white/[0.06] shrink-0">
          <button onClick={() => onSetView(view === 'settings' ? 'chat' : 'settings')}
            className={`w-full flex items-center gap-2.5 px-3 py-2 rounded-xl text-[13px] font-medium transition-all duration-150 cursor-pointer
              ${view === 'settings' ? 'au-btn-active' : 'text-[#8893A6] hover:bg-white/[0.04] hover:text-[#ECEFF4] border border-transparent'}`}>
            <IconSettings /> Settings
          </button>
        </div>
      </aside>

      {/* ── Main ── */}
      <main className="relative z-10 flex-1 flex flex-col h-full min-w-0">
        {view === 'settings' ? (
          <div className="au-view-enter flex flex-col h-full">
            <AuroraSettingsView themes={themes} activeTheme={activeTheme} onSetTheme={onSetTheme} />
          </div>
        ) : !activePeer ? (
          <div className="au-view-enter flex flex-col h-full">
            <AuroraEmptyState />
          </div>
        ) : (
          <div className="au-view-enter flex flex-col h-full">
            {/* Header */}
            <header className="au-frost drag-region pt-11 px-6 pb-3 border-b border-white/[0.06] shrink-0">
              <div className="flex items-center justify-between">
                <div>
                  <h2 className="text-[15px] font-semibold text-[#ECEFF4] tracking-tight">{activePeer}</h2>
                  <div className="flex items-center gap-2 mt-0.5">
                    <span className="inline-flex items-center gap-1 text-[#88C0D0] text-[10px] font-medium" style={{ fontFamily: "'IBM Plex Mono', monospace" }}><IconLock /> E2E</span>
                    {connected[activePeer] && <span className="text-[#A3BE8C] text-[10px] flex items-center gap-1" style={{ fontFamily: "'IBM Plex Mono', monospace" }}><span className="w-1.5 h-1.5 rounded-full bg-[#A3BE8C]" /> Connected</span>}
                    {typing[activePeer] && <span className="text-[#8FBCBB] text-[11px] italic font-light">typing...</span>}
                  </div>
                </div>
                {activePeerData && (
                  <div className="text-[11px] text-[#5D6882]" style={{ fontFamily: "'IBM Plex Mono', monospace" }}>
                    {activePeerData.OS}
                    {activePeerData.RunningTailchat && <span className="inline-flex items-center gap-0.5 ml-1.5 text-[#81A1C1]"><IconShield /> tailchat</span>}
                  </div>
                )}
              </div>
            </header>

            {/* Messages */}
            <div className="flex-1 overflow-y-auto px-6 py-4">
              {messages.length === 0 && (
                <div className="flex items-center justify-center h-full">
                  <div className="text-center">
                    <div className="au-glass w-10 h-10 rounded-xl flex items-center justify-center mx-auto mb-3">
                      <span className="text-[#88C0D0] opacity-40"><IconLock /></span>
                    </div>
                    <p className="text-[#8893A6] text-[13px] font-medium">No messages yet</p>
                    <p className="text-[#5D6882] text-[11px] mt-0.5">End-to-end encrypted</p>
                  </div>
                </div>
              )}
              {groupMessages(messages).map((g, i) => (
                <AuroraMessageGroup key={i} group={g} onReaction={onReaction} />
              ))}
              <div ref={messagesEndRef} />
            </div>

            {/* Input */}
            <div className="au-frost px-6 py-3 border-t border-white/[0.06] shrink-0">
              <div className="flex items-end gap-2.5">
                <button onClick={onOpenGifs}
                  className="au-glass shrink-0 w-9 h-9 rounded-xl text-[#5D6882] hover:text-[#88C0D0] flex items-center justify-center transition-all duration-150 cursor-pointer text-[11px] font-bold" style={{ fontFamily: "'IBM Plex Mono', monospace" }}>
                  GIF
                </button>
                <textarea ref={textareaRef} placeholder={`Message ${activePeer}...`} value={inputText}
                  onChange={onInputChange} onKeyDown={onInputKeyDown} rows={1}
                  className="au-input-frost flex-1 rounded-xl px-4 py-2.5 text-[14px] min-h-[40px] max-h-40 leading-relaxed resize-none" />
                <button onClick={onSend} disabled={!inputText.trim()}
                  className={`shrink-0 w-9 h-9 rounded-full flex items-center justify-center transition-all duration-200 cursor-pointer
                    ${inputText.trim() ? 'bg-[#88C0D0] text-[#2E3440] shadow-[0_0_16px_rgba(136,192,208,0.3)]' : 'au-glass text-[#5D6882] cursor-not-allowed'}`}>
                  <IconSend />
                </button>
              </div>
            </div>
          </div>
        )}
      </main>

      {/* GIF picker */}
      {showGifPicker && (
        <div className="fixed inset-0 bg-black/40 backdrop-blur-sm z-50 flex items-end justify-center" onClick={onCloseGifs}>
          <div className="au-frost w-[480px] max-h-[440px] border border-white/[0.08] rounded-t-2xl flex flex-col shadow-2xl au-slide-up" onClick={e => e.stopPropagation()}>
            <div className="p-3 border-b border-white/[0.06] shrink-0">
              <div className="relative">
                <div className="absolute left-2.5 top-1/2 -translate-y-1/2 text-[#5D6882]"><IconSearch /></div>
                <input type="text" placeholder="Search GIFs..." value={gifQuery} onChange={e => onSearchGifs(e.target.value)} autoFocus
                  className="au-input-frost w-full pl-8 pr-3 py-2 text-[13px] rounded-xl" />
              </div>
            </div>
            <div className="flex-1 overflow-y-auto p-2 grid grid-cols-3 gap-1.5">
              {gifLoading && <p className="col-span-3 text-center py-8 text-[#5D6882] text-[12px]">Loading...</p>}
              {!gifLoading && gifResults.map(gif => (
                <div key={gif.ID} onClick={() => onPickGif(gif)} className="aspect-square rounded-xl overflow-hidden cursor-pointer au-glass hover:scale-[1.03] transition-transform duration-150">
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

function AuroraPeerItem({ peer, active, unread, connected, onClick }: { peer: Peer; active: boolean; unread: number; connected: boolean; onClick: () => void }) {
  return (
    <div onClick={onClick}
      className={`au-peer-item flex items-center gap-2.5 px-2.5 py-[9px] rounded-xl cursor-pointer mb-0.5
        ${active ? 'au-peer-active' : 'hover:bg-white/[0.04]'} border ${active ? 'border-[rgba(136,192,208,0.18)]' : 'border-transparent'}`}>
      <div className="relative shrink-0 w-[34px] h-[34px] rounded-full au-glass flex items-center justify-center text-[12px] font-semibold" style={{ fontFamily: "'IBM Plex Mono', monospace", color: active ? '#88C0D0' : '#8893A6' }}>
        {initials(peer.Hostname)}
        <span className={`absolute -bottom-0.5 -right-0.5 w-[10px] h-[10px] rounded-full border-2 border-[#3B4252]
          ${peer.Online ? peer.RunningTailchat ? 'bg-[#A3BE8C] shadow-[0_0_6px_rgba(163,190,140,0.5)]' : 'bg-[#A3BE8C]' : 'bg-[#5D6882]'}`} />
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-1.5">
          <span className="text-[13px] font-medium text-[#ECEFF4] truncate">{peer.Hostname}</span>
          {peer.RunningTailchat && <span className="text-[#81A1C1] shrink-0"><IconShield /></span>}
        </div>
        <div className="text-[11px] text-[#5D6882] truncate" style={{ fontFamily: "'IBM Plex Mono', monospace" }}>{peer.OS}{connected ? ' · connected' : ''}</div>
      </div>
      {unread > 0 && <span className="bg-[#88C0D0] text-[#2E3440] text-[10px] font-bold min-w-[18px] h-[18px] rounded-full flex items-center justify-center px-1 shadow-[0_0_10px_rgba(136,192,208,0.3)]">{unread}</span>}
    </div>
  );
}

function AuroraMessageGroup({ group, onReaction }: { group: MsgGroup; onReaction: (id: string, emoji: string) => void }) {
  return (
    <div className="au-msg-row mb-3">
      <div className="flex items-center gap-2 mb-1 pl-11">
        <span className={`text-[13px] font-semibold ${group.isOwn ? 'text-[#88C0D0]' : 'text-[#8FBCBB]'}`}>{group.sender}</span>
        <span className="text-[10px] text-[#5D6882]" style={{ fontFamily: "'IBM Plex Mono', monospace" }}>{group.time}</span>
      </div>
      {group.messages.map(msg => <AuroraMessageRow key={msg.ID} msg={msg} onReaction={onReaction} />)}
    </div>
  );
}

function AuroraMessageRow({ msg, onReaction }: { msg: Message; onReaction: (id: string, emoji: string) => void }) {
  const words = msg.Content.trim().split(/\s+/);
  const isGif = words.length === 1 && isImageURL(words[0]);
  const reactions = new Map<string, string[]>();
  if (msg.Reactions) for (const r of msg.Reactions) reactions.set(r.Emoji, [...(reactions.get(r.Emoji) || []), r.Sender]);

  return (
    <div className="group/msg relative flex items-start gap-2.5 px-2 py-[3px] -mx-2 rounded-lg hover:bg-white/[0.02] transition-colors duration-100">
      <div className="w-[28px] shrink-0" />
      <div className="flex-1 min-w-0">
        {isGif ? (
          <div className="rounded-xl overflow-hidden max-w-[280px] my-0.5 border border-white/[0.06]">
            <img src={words[0]} alt="GIF" className="block w-full h-auto" />
          </div>
        ) : <p className="text-[13.5px] text-[#D8DEE9] leading-[1.55] break-words">{msg.Content}</p>}
        {msg.IsOwn && <span className="text-[10px]" style={{ fontFamily: "'IBM Plex Mono', monospace", color: msg.State === 2 ? '#81A1C1' : '#5D6882' }}>{msg.State === 2 ? 'Read' : msg.State === 1 ? 'Delivered' : 'Sending...'}</span>}
        {reactions.size > 0 && (
          <div className="flex gap-1 mt-1 flex-wrap">
            {Array.from(reactions.entries()).map(([emoji, senders]) => (
              <button key={emoji} onClick={() => onReaction(msg.ID, emoji)} title={senders.join(', ')}
                className="au-glass inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[12px] hover:border-[rgba(136,192,208,0.2)] transition-colors duration-100 cursor-pointer border border-white/[0.06]">
                {emoji} <span className="text-[10px] text-[#5D6882]" style={{ fontFamily: "'IBM Plex Mono', monospace" }}>{senders.length}</span>
              </button>
            ))}
          </div>
        )}
      </div>
      <div className="au-msg-actions absolute -top-3 right-2 au-frost flex items-center gap-0.5 border border-white/[0.08] rounded-lg shadow-lg px-0.5 py-0.5">
        {QUICK_REACTIONS.map(emoji => <button key={emoji} onClick={() => onReaction(msg.ID, emoji)} className="w-6 h-6 flex items-center justify-center rounded text-[13px] hover:bg-white/[0.06] transition-colors cursor-pointer">{emoji}</button>)}
      </div>
    </div>
  );
}

function AuroraEmptyState() {
  return (
    <>
      <header className="au-frost drag-region pt-11 px-6 pb-3 border-b border-white/[0.06] shrink-0">
        <div className="flex items-center gap-2">
          <span className="au-icon-frost"><IconSnowflake /></span>
          <h2 className="text-[15px] font-semibold text-[#ECEFF4] tracking-tight">aurora</h2>
        </div>
      </header>
      <div className="flex-1 flex flex-col items-center justify-center text-center px-12">
        <div className="relative mb-6">
          <div className="au-glass w-16 h-16 rounded-2xl flex items-center justify-center" style={{ boxShadow: '0 0 40px rgba(136,192,208,0.08)' }}>
            <span className="text-[#88C0D0] opacity-35">
              <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"><path d="M12 2v20M17 7l-5-5-5 5M17 17l-5 5-5-5M2 12h20M7 7L2 12l5 5M17 7l5 5-5 5"/></svg>
            </span>
          </div>
          <div className="absolute -inset-4 rounded-3xl bg-[radial-gradient(circle,rgba(136,192,208,0.06),transparent_70%)] pointer-events-none" />
        </div>
        <h3 className="text-[16px] font-semibold text-[#ECEFF4] mb-1.5">Select a peer to start chatting</h3>
        <p className="text-[12px] text-[#5D6882] max-w-[280px] leading-relaxed">All messages are end-to-end encrypted using X25519 key exchange and XChaCha20-Poly1305.</p>
        <div className="mt-4 flex items-center gap-2 text-[#5D6882]">
          <span className="w-8 h-px bg-white/[0.06]" />
          <span className="text-[10px] uppercase tracking-wider text-[#5E81AC] opacity-60" style={{ fontFamily: "'IBM Plex Mono', monospace" }}>Nord Frost</span>
          <span className="w-8 h-px bg-white/[0.06]" />
        </div>
      </div>
    </>
  );
}

function AuroraSettingsView({ themes, activeTheme, onSetTheme }: { themes: ThemeInfo[]; activeTheme: string; onSetTheme: (name: string) => void }) {
  return (
    <>
      <header className="au-frost drag-region pt-11 px-6 pb-3 border-b border-white/[0.06] shrink-0">
        <h2 className="text-[15px] font-semibold text-[#ECEFF4] tracking-tight">Settings</h2>
        <p className="text-[11px] text-[#5D6882] mt-0.5" style={{ fontFamily: "'IBM Plex Mono', monospace" }}>Configuration & themes</p>
      </header>
      <div className="flex-1 overflow-y-auto px-6 py-6">
        <section>
          <h3 className="text-[13px] font-semibold text-[#ECEFF4] mb-1">Themes</h3>
          <p className="text-[12px] text-[#5D6882] mb-4 leading-relaxed">
            Choose a theme or install custom ones at <code className="text-[#88C0D0] au-glass px-1.5 py-0.5 rounded text-[11px] border border-white/[0.06]" style={{ fontFamily: "'IBM Plex Mono', monospace" }}>~/.tailchat/themes/</code>
          </p>
          <div className="space-y-2">
            {themes.map(t => (
              <div key={t.name} onClick={() => onSetTheme(t.name)}
                className={`group flex items-center gap-4 p-3.5 rounded-xl cursor-pointer border transition-all duration-150
                  ${t.name === activeTheme ? 'au-peer-active border-[rgba(136,192,208,0.18)]' : 'au-glass border-white/[0.06] hover:border-[rgba(136,192,208,0.12)]'}`}>
                <div className="au-glass w-8 h-8 rounded-lg shrink-0 flex items-center justify-center text-[14px] border border-white/[0.06]">
                  {t.name === 'aurora' ? '❄️' : t.name === 'default' ? '🌑' : '🎨'}
                </div>
                <div className="flex-1 min-w-0">
                  <div className="text-[13px] font-medium text-[#ECEFF4]">{t.name}</div>
                  {t.description && <div className="text-[11px] text-[#5D6882] mt-0.5">{t.description}</div>}
                  {t.author && <div className="text-[10px] text-[#5D6882] mt-0.5 opacity-50" style={{ fontFamily: "'IBM Plex Mono', monospace" }}>by {t.author}</div>}
                </div>
                {t.name === activeTheme && <span className="text-[#88C0D0] text-[10px] font-semibold uppercase tracking-wider shrink-0" style={{ fontFamily: "'IBM Plex Mono', monospace" }}>Active</span>}
              </div>
            ))}
          </div>
        </section>
        <section className="mt-8">
          <h3 className="text-[13px] font-semibold text-[#ECEFF4] mb-3">Creating a theme</h3>
          <div className="au-glass border border-white/[0.06] rounded-xl p-5 text-[12px] text-[#8893A6] leading-[1.7] space-y-2.5">
            <p>Create a folder in <code className="text-[#88C0D0] text-[11px] bg-[#242933] px-1 py-0.5 rounded" style={{ fontFamily: "'IBM Plex Mono', monospace" }}>~/.tailchat/themes/my-theme/</code> with an <code className="text-[#88C0D0] text-[11px] bg-[#242933] px-1 py-0.5 rounded" style={{ fontFamily: "'IBM Plex Mono', monospace" }}>index.html</code>.</p>
            <p>Your theme has full access to the Go backend via <code className="text-[#88C0D0] text-[11px] bg-[#242933] px-1 py-0.5 rounded" style={{ fontFamily: "'IBM Plex Mono', monospace" }}>window.go.main.App.*</code> and <code className="text-[#88C0D0] text-[11px] bg-[#242933] px-1 py-0.5 rounded" style={{ fontFamily: "'IBM Plex Mono', monospace" }}>window.runtime.*</code>.</p>
          </div>
        </section>
      </div>
    </>
  );
}
