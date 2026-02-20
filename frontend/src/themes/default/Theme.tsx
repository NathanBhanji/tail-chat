import type { ThemeProps, Message, Peer, GIF, ThemeInfo } from '../types';

// ─── Helpers ────────────────────────────────────────────────────────

const isImageURL = (s: string) => {
  const u = s.trim().toLowerCase();
  return u.includes('tenor.com') || u.includes('giphy.com') || /\.(gif|png|jpg|jpeg|webp)(\?.*)?$/.test(u);
};

const formatTime = (ts: string) =>
  new Date(ts).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });

const initials = (h: string) => h.slice(0, 2).toUpperCase();

const QUICK_REACTIONS = ['👍', '😂', '🔥', '❤️', '👀'];

// ─── Icons ──────────────────────────────────────────────────────────

const IconSearch = () => (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
    <circle cx="11" cy="11" r="8"/><path d="m21 21-4.35-4.35"/>
  </svg>
);

const IconShield = () => (
  <svg width="12" height="12" viewBox="0 0 24 24" fill="currentColor" opacity="0.7">
    <path d="M12 2L3 7v5c0 5.55 3.84 10.74 9 12 5.16-1.26 9-6.45 9-12V7l-9-5z"/>
  </svg>
);

const IconSettings = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
    <circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>
  </svg>
);

const IconSend = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M5 12h14M12 5l7 7-7 7"/>
  </svg>
);

const IconGif = () => (
  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
    <rect x="2" y="4" width="20" height="16" rx="3"/><text x="12" y="15" textAnchor="middle" fill="currentColor" stroke="none" fontSize="8" fontWeight="700" fontFamily="var(--font-mono)">GIF</text>
  </svg>
);

const IconLock = () => (
  <svg width="11" height="11" viewBox="0 0 24 24" fill="currentColor">
    <path d="M18 8h-1V6c0-2.76-2.24-5-5-5S7 3.24 7 6v2H6c-1.1 0-2 .9-2 2v10c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V10c0-1.1-.9-2-2-2zM12 17c-1.1 0-2-.9-2-2s.9-2 2-2 2 .9 2 2-.9 2-2 2zM9 8V6c0-1.66 1.34-3 3-3s3 1.34 3 3v2H9z"/>
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

// ─── Default Theme ("Cipher Lounge") ────────────────────────────────

export default function DefaultTheme(props: ThemeProps) {
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
    <div className="flex h-screen w-screen" style={{ background: 'var(--color-tc-base)', fontFamily: 'var(--font-sans)' }}>
      {/* ── Sidebar ── */}
      <aside className="w-[272px] min-w-[272px] bg-tc-surface flex flex-col h-full border-r border-tc-border-subtle">
        {/* Brand */}
        <div className="drag-region relative pt-11 px-5 pb-3 shrink-0">
          <div className="absolute top-0 left-0 w-full h-full bg-[radial-gradient(ellipse_at_20%_20%,oklch(0.80_0.20_160/0.04),transparent_70%)] pointer-events-none" />
          <div className="relative">
            <div className="flex items-center gap-0">
              <span className="font-mono text-tc-accent text-[14px] font-medium mr-1 opacity-60">//</span>
              <h1 className="font-mono text-[16px] font-semibold tracking-tight text-tc-text">tailchat</h1>
            </div>
            <div className="flex items-center gap-1.5 mt-1">
              <span className="w-[6px] h-[6px] rounded-full bg-tc-online shrink-0" style={{ animation: 'glow-pulse 3s ease-in-out infinite', boxShadow: '0 0 6px oklch(0.80 0.20 160 / 0.5)' }} />
              <span className="font-mono text-[11px] text-tc-text-tertiary">{selfInfo.hostname} &middot; {selfInfo.ip}</span>
            </div>
          </div>
        </div>

        {/* Search */}
        <div className="px-4 py-2 shrink-0">
          <div className="relative">
            <div className="absolute left-2.5 top-1/2 -translate-y-1/2 text-tc-text-tertiary"><IconSearch /></div>
            <input type="text" placeholder="Search peers..." value={searchQuery}
              onChange={e => onSearchChange(e.target.value)}
              className="w-full bg-tc-elevated border border-tc-border-subtle rounded-lg pl-8 pr-3 py-[7px] text-[13px] text-tc-text placeholder:text-tc-text-tertiary outline-none focus:border-tc-accent-border focus:bg-tc-hover transition-all duration-150" />
          </div>
        </div>

        {/* Peers */}
        <div className="flex-1 overflow-y-auto px-3 py-1 space-y-0.5">
          {online.length > 0 && (
            <>
              <div className="font-mono text-[10px] font-medium uppercase tracking-[0.1em] text-tc-text-tertiary px-2 pt-4 pb-1.5">Online &mdash; {online.length}</div>
              {online.map(p => <DefaultPeerItem key={p.Hostname} peer={p} active={p.Hostname === activePeer} unread={unread[p.Hostname] || 0} connected={connected[p.Hostname] || false} onClick={() => onSelectPeer(p)} />)}
            </>
          )}
          {offline.length > 0 && (
            <>
              <div className="font-mono text-[10px] font-medium uppercase tracking-[0.1em] text-tc-text-tertiary px-2 pt-4 pb-1.5">Offline &mdash; {offline.length}</div>
              {offline.map(p => <DefaultPeerItem key={p.Hostname} peer={p} active={p.Hostname === activePeer} unread={0} connected={false} onClick={() => onSelectPeer(p)} />)}
            </>
          )}
          {filtered.length === 0 && <p className="px-2 py-6 text-tc-text-tertiary text-[12px] text-center">No peers found.</p>}
        </div>

        {/* Settings */}
        <div className="px-3 py-2.5 border-t border-tc-border-subtle shrink-0">
          <button onClick={() => onSetView(view === 'settings' ? 'chat' : 'settings')}
            className={`w-full flex items-center gap-2.5 px-3 py-2 rounded-lg text-[13px] font-medium transition-all duration-150 cursor-pointer
              ${view === 'settings' ? 'bg-tc-accent-bg text-tc-accent border border-tc-accent-border' : 'text-tc-text-secondary hover:bg-tc-hover hover:text-tc-text border border-transparent'}`}>
            <IconSettings /> Settings
          </button>
        </div>
      </aside>

      {/* ── Main ── */}
      <main className="flex-1 flex flex-col h-full min-w-0">
        {view === 'settings' ? (
          <div className="view-enter flex flex-col h-full">
            <DefaultSettingsView themes={themes} activeTheme={activeTheme} onSetTheme={onSetTheme} />
          </div>
        ) : !activePeer ? (
          <div className="view-enter flex flex-col h-full">
            <DefaultEmptyState />
          </div>
        ) : (
          <div className="view-enter flex flex-col h-full">
            {/* Header */}
            <header className="drag-region pt-11 px-6 pb-3 border-b border-tc-border-subtle bg-tc-surface/50 backdrop-blur-sm shrink-0">
              <div className="flex items-center justify-between">
                <div>
                  <h2 className="text-[15px] font-semibold text-tc-text tracking-tight">{activePeer}</h2>
                  <div className="flex items-center gap-2 mt-0.5">
                    <span className="inline-flex items-center gap-1 text-tc-accent text-[10px] font-mono font-medium"><IconLock /> E2E</span>
                    {connected[activePeer] && <span className="text-tc-online text-[10px] font-mono flex items-center gap-1"><span className="w-1.5 h-1.5 rounded-full bg-tc-online" /> Connected</span>}
                    {typing[activePeer] && <span className="text-tc-accent text-[11px] italic font-light">typing...</span>}
                  </div>
                </div>
                {activePeerData && (
                  <div className="text-[11px] font-mono text-tc-text-tertiary">
                    {activePeerData.OS}
                    {activePeerData.RunningTailchat && <span className="inline-flex items-center gap-0.5 ml-1.5 text-tc-accent-dim"><IconShield /> tailchat</span>}
                  </div>
                )}
              </div>
            </header>

            {/* Messages */}
            <div className="flex-1 overflow-y-auto px-6 py-4">
              {messages.length === 0 && (
                <div className="flex items-center justify-center h-full">
                  <div className="text-center">
                    <div className="w-10 h-10 rounded-xl bg-tc-elevated border border-tc-border-subtle flex items-center justify-center mx-auto mb-3">
                      <span className="text-tc-accent opacity-50"><IconLock /></span>
                    </div>
                    <p className="text-tc-text-secondary text-[13px] font-medium">No messages yet</p>
                    <p className="text-tc-text-tertiary text-[11px] mt-0.5">Messages are end-to-end encrypted</p>
                  </div>
                </div>
              )}
              {groupMessages(messages).map((g, i) => (
                <DefaultMessageGroup key={i} group={g} onReaction={onReaction} />
              ))}
              <div ref={messagesEndRef} />
            </div>

            {/* Input */}
            <div className="px-6 py-3 border-t border-tc-border-subtle bg-tc-surface/30 shrink-0">
              <div className="flex items-end gap-2">
                <button onClick={onOpenGifs} className="shrink-0 w-9 h-9 rounded-lg bg-tc-elevated border border-tc-border-subtle text-tc-text-tertiary hover:text-tc-accent hover:border-tc-accent-border flex items-center justify-center transition-all duration-150 cursor-pointer"><IconGif /></button>
                <textarea ref={textareaRef} placeholder={`Message ${activePeer}...`} value={inputText}
                  onChange={onInputChange} onKeyDown={onInputKeyDown} rows={1}
                  className="flex-1 bg-tc-elevated border border-tc-border-subtle rounded-xl px-4 py-2.5 text-[14px] text-tc-text placeholder:text-tc-text-tertiary outline-none resize-none min-h-[40px] max-h-40 leading-relaxed focus:border-tc-accent-border focus:bg-tc-hover transition-all duration-150" />
                <button onClick={onSend} disabled={!inputText.trim()}
                  className={`shrink-0 w-9 h-9 rounded-full flex items-center justify-center transition-all duration-150 cursor-pointer
                    ${inputText.trim() ? 'bg-tc-accent text-tc-base hover:bg-tc-accent-dim shadow-[0_0_12px_oklch(0.80_0.20_160/0.3)]' : 'bg-tc-elevated border border-tc-border-subtle text-tc-text-tertiary cursor-not-allowed'}`}>
                  <IconSend />
                </button>
              </div>
            </div>
          </div>
        )}
      </main>

      {/* GIF picker */}
      {showGifPicker && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-end justify-center" onClick={onCloseGifs}>
          <div className="w-[480px] max-h-[440px] bg-tc-surface border border-tc-border rounded-t-2xl flex flex-col shadow-2xl" style={{ animation: 'slide-up 0.2s ease-out' }} onClick={e => e.stopPropagation()}>
            <div className="p-3 border-b border-tc-border-subtle shrink-0">
              <div className="relative">
                <div className="absolute left-2.5 top-1/2 -translate-y-1/2 text-tc-text-tertiary"><IconSearch /></div>
                <input type="text" placeholder="Search GIFs..." value={gifQuery} onChange={e => onSearchGifs(e.target.value)} autoFocus
                  className="w-full bg-tc-elevated border border-tc-border-subtle rounded-lg pl-8 pr-3 py-2 text-[13px] text-tc-text placeholder:text-tc-text-tertiary outline-none focus:border-tc-accent-border transition-colors" />
              </div>
            </div>
            <div className="flex-1 overflow-y-auto p-2 grid grid-cols-3 gap-1.5">
              {gifLoading && <p className="col-span-3 text-center py-8 text-tc-text-tertiary text-[12px]">Loading...</p>}
              {!gifLoading && gifResults.map(gif => (
                <div key={gif.ID} onClick={() => onPickGif(gif)} className="aspect-square rounded-lg overflow-hidden cursor-pointer bg-tc-elevated hover:scale-[1.03] transition-transform duration-150">
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

function DefaultPeerItem({ peer, active, unread, connected, onClick }: { peer: Peer; active: boolean; unread: number; connected: boolean; onClick: () => void }) {
  return (
    <div onClick={onClick} className={`peer-item flex items-center gap-2.5 px-2.5 py-[9px] rounded-lg cursor-pointer border-l-2
      ${active ? 'bg-tc-accent-bg border-l-tc-accent' : 'border-l-transparent hover:bg-tc-hover'}`}>
      <div className="relative shrink-0 w-[34px] h-[34px] rounded-full flex items-center justify-center font-mono text-[12px] font-semibold"
        style={{ background: active ? 'linear-gradient(135deg, oklch(0.80 0.20 160 / 0.15), oklch(0.80 0.20 160 / 0.05))' : 'linear-gradient(135deg, #1e1e28, #16161e)', color: active ? 'var(--color-tc-accent)' : 'var(--color-tc-text-secondary)' }}>
        {initials(peer.Hostname)}
        <span className={`absolute -bottom-0.5 -right-0.5 w-[10px] h-[10px] rounded-full border-[2px] border-tc-surface ${peer.RunningTailchat && peer.Online ? 'bg-tc-online shadow-[0_0_6px_oklch(0.80_0.20_160/0.5)]' : peer.Online ? 'bg-tc-online' : 'bg-tc-offline'}`} />
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-1.5">
          <span className="text-[13px] font-medium text-tc-text truncate">{peer.Hostname}</span>
          {peer.RunningTailchat && <span className="text-tc-accent-dim shrink-0"><IconShield /></span>}
        </div>
        <div className="text-[11px] text-tc-text-tertiary font-mono truncate">{peer.OS}{connected ? ' · connected' : ''}</div>
      </div>
      {unread > 0 && <span className="bg-tc-accent text-tc-base text-[10px] font-bold min-w-[18px] h-[18px] rounded-full flex items-center justify-center px-1 shadow-[0_0_8px_oklch(0.80_0.20_160/0.4)]">{unread}</span>}
    </div>
  );
}

function DefaultMessageGroup({ group, onReaction }: { group: MsgGroup; onReaction: (id: string, emoji: string) => void }) {
  return (
    <div className="msg-row mb-3">
      <div className="flex items-center gap-2 mb-1 pl-11">
        <span className={`text-[13px] font-semibold ${group.isOwn ? 'text-tc-accent' : 'text-tc-info'}`}>{group.sender}</span>
        <span className="text-[10px] font-mono text-tc-text-tertiary">{group.time}</span>
      </div>
      {group.messages.map(msg => <DefaultMessageRow key={msg.ID} msg={msg} onReaction={onReaction} />)}
    </div>
  );
}

function DefaultMessageRow({ msg, onReaction }: { msg: Message; onReaction: (id: string, emoji: string) => void }) {
  const words = msg.Content.trim().split(/\s+/);
  const isGif = words.length === 1 && isImageURL(words[0]);
  const reactions = new Map<string, string[]>();
  if (msg.Reactions) for (const r of msg.Reactions) reactions.set(r.Emoji, [...(reactions.get(r.Emoji) || []), r.Sender]);

  return (
    <div className="group/msg relative flex items-start gap-2.5 px-2 py-[3px] -mx-2 rounded-md hover:bg-tc-msg-hover transition-colors duration-100">
      <div className="w-[28px] shrink-0" />
      <div className="flex-1 min-w-0">
        {isGif ? (
          <div className="rounded-lg overflow-hidden max-w-[280px] hover:scale-[1.01] transition-transform duration-150 cursor-pointer my-0.5">
            <img src={words[0]} alt="GIF" className="block w-full h-auto rounded-lg" />
          </div>
        ) : <p className="text-[13.5px] text-tc-text leading-[1.55] break-words">{msg.Content}</p>}
        {msg.IsOwn && <span className={`font-mono text-[10px] ${msg.State === 2 ? 'text-tc-accent-dim' : msg.State === 1 ? 'text-tc-text-tertiary' : 'text-tc-text-tertiary opacity-50'}`}>{msg.State === 2 ? 'Read' : msg.State === 1 ? 'Delivered' : 'Sending...'}</span>}
        {reactions.size > 0 && (
          <div className="flex gap-1 mt-1 flex-wrap">
            {Array.from(reactions.entries()).map(([emoji, senders]) => (
              <button key={emoji} onClick={() => onReaction(msg.ID, emoji)} title={senders.join(', ')} className="inline-flex items-center gap-1 bg-tc-elevated border border-tc-border-subtle rounded-full px-2 py-0.5 text-[12px] hover:border-tc-accent-border transition-colors duration-100 cursor-pointer">
                {emoji} <span className="font-mono text-[10px] text-tc-text-tertiary">{senders.length}</span>
              </button>
            ))}
          </div>
        )}
      </div>
      <div className="msg-actions absolute -top-3 right-2 flex items-center gap-0.5 bg-tc-elevated border border-tc-border rounded-md shadow-lg px-0.5 py-0.5 opacity-0 group-hover/msg:opacity-100 transition-opacity duration-100">
        {QUICK_REACTIONS.map(emoji => <button key={emoji} onClick={() => onReaction(msg.ID, emoji)} className="w-6 h-6 flex items-center justify-center rounded text-[13px] hover:bg-tc-hover transition-colors cursor-pointer" title={`React with ${emoji}`}>{emoji}</button>)}
      </div>
    </div>
  );
}

function DefaultEmptyState() {
  return (
    <>
      <header className="drag-region pt-11 px-6 pb-3 border-b border-tc-border-subtle bg-tc-surface/50 shrink-0">
        <div className="flex items-center gap-0">
          <span className="font-mono text-tc-accent text-[14px] font-medium mr-1 opacity-60">//</span>
          <h2 className="font-mono text-[15px] font-semibold text-tc-text tracking-tight">tailchat</h2>
        </div>
      </header>
      <div className="flex-1 flex flex-col items-center justify-center text-center px-12">
        <div className="relative mb-6">
          <div className="w-16 h-16 rounded-2xl bg-tc-elevated border border-tc-border-subtle flex items-center justify-center" style={{ boxShadow: '0 0 40px oklch(0.80 0.20 160 / 0.05)' }}>
            <span className="text-tc-accent text-2xl opacity-40">
              <svg width="28" height="28" viewBox="0 0 24 24" fill="currentColor"><path d="M18 8h-1V6c0-2.76-2.24-5-5-5S7 3.24 7 6v2H6c-1.1 0-2 .9-2 2v10c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V10c0-1.1-.9-2-2-2zM12 17c-1.1 0-2-.9-2-2s.9-2 2-2 2 .9 2 2-.9 2-2 2zM9 8V6c0-1.66 1.34-3 3-3s3 1.34 3 3v2H9z"/></svg>
            </span>
          </div>
          <div className="absolute -inset-3 rounded-3xl bg-[radial-gradient(circle,oklch(0.80_0.20_160/0.06),transparent_70%)] pointer-events-none" />
        </div>
        <h3 className="text-[16px] font-semibold text-tc-text mb-1.5 tracking-tight">Select a peer to start chatting</h3>
        <p className="text-[12px] text-tc-text-tertiary max-w-[280px] leading-relaxed">All messages are end-to-end encrypted using X25519 key exchange and XChaCha20-Poly1305.</p>
        <div className="mt-4 flex items-center gap-1.5 text-tc-text-tertiary">
          <span className="w-8 h-px bg-tc-border" /><span className="text-[10px] font-mono uppercase tracking-wider">Zero Trust</span><span className="w-8 h-px bg-tc-border" />
        </div>
      </div>
    </>
  );
}

function DefaultSettingsView({ themes, activeTheme, onSetTheme }: { themes: ThemeInfo[]; activeTheme: string; onSetTheme: (name: string) => void }) {
  return (
    <>
      <header className="drag-region pt-11 px-6 pb-3 border-b border-tc-border-subtle bg-tc-surface/50 shrink-0">
        <h2 className="text-[15px] font-semibold text-tc-text tracking-tight">Settings</h2>
        <p className="text-[11px] text-tc-text-tertiary mt-0.5 font-mono">Configuration & themes</p>
      </header>
      <div className="flex-1 overflow-y-auto px-6 py-6">
        <section>
          <h3 className="text-[13px] font-semibold text-tc-text mb-1 tracking-tight">Themes</h3>
          <p className="text-[12px] text-tc-text-tertiary mb-4 leading-relaxed">
            Choose a theme or install custom ones at <code className="font-mono text-tc-accent bg-tc-elevated px-1.5 py-0.5 rounded text-[11px] border border-tc-border-subtle">~/.tailchat/themes/</code>
          </p>
          <div className="space-y-2">
            {themes.map(t => (
              <div key={t.name} onClick={() => onSetTheme(t.name)}
                className={`group flex items-center gap-4 p-3.5 rounded-xl cursor-pointer border transition-all duration-150
                  ${t.name === activeTheme ? 'bg-tc-accent-bg border-tc-accent-border' : 'bg-tc-elevated border-tc-border-subtle hover:border-tc-accent-border hover:bg-tc-hover'}`}>
                <div className={`w-8 h-8 rounded-lg shrink-0 flex items-center justify-center text-[14px] ${t.name === activeTheme ? 'bg-tc-accent/20' : 'bg-tc-hover'}`}>
                  {t.name === 'default' ? '🌑' : t.name === 'aurora' ? '❄️' : '🎨'}
                </div>
                <div className="flex-1 min-w-0">
                  <div className="text-[13px] font-medium text-tc-text">{t.name}</div>
                  {t.description && <div className="text-[11px] text-tc-text-tertiary mt-0.5">{t.description}</div>}
                  {t.author && <div className="text-[10px] text-tc-text-tertiary font-mono mt-0.5 opacity-60">by {t.author}</div>}
                </div>
                {t.name === activeTheme && <span className="text-tc-accent text-[10px] font-mono font-semibold uppercase tracking-wider shrink-0">Active</span>}
              </div>
            ))}
          </div>
        </section>
        <section className="mt-8">
          <h3 className="text-[13px] font-semibold text-tc-text mb-3 tracking-tight">Creating a theme</h3>
          <div className="bg-tc-elevated border border-tc-border-subtle rounded-xl p-5 text-[12px] text-tc-text-secondary leading-[1.7] space-y-2.5">
            <p>Create a folder in <code className="font-mono text-tc-accent text-[11px] bg-tc-base px-1 py-0.5 rounded">~/.tailchat/themes/my-theme/</code> with an <code className="font-mono text-tc-accent text-[11px] bg-tc-base px-1 py-0.5 rounded">index.html</code>.</p>
            <p>Your theme has full access to the Go backend via <code className="font-mono text-tc-accent text-[11px] bg-tc-base px-1 py-0.5 rounded">window.go.main.App.*</code> and <code className="font-mono text-tc-accent text-[11px] bg-tc-base px-1 py-0.5 rounded">window.runtime.*</code>.</p>
          </div>
        </section>
      </div>
    </>
  );
}
