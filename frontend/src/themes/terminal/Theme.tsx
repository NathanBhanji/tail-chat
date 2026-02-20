import { useState } from 'react';
import type { ThemeProps, Message, Peer, ThemeInfo } from '../types';
import './terminal.css';

// ─── Helpers ────────────────────────────────────────────────────────

const isImageURL = (s: string) => {
  const u = s.trim().toLowerCase();
  return u.includes('tenor.com') || u.includes('giphy.com') || /\.(gif|png|jpg|jpeg|webp)(\?.*)?$/.test(u);
};
const formatTime = (ts: string) => new Date(ts).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', hour12: false });
const QUICK_REACTIONS = ['👍', '😂', '🔥', '❤️', '✨'];

// ─── Terminal Theme ("Green Phosphor CRT") ──────────────────────────
// Layout: Full-screen terminal. Status bar → peer bar → scrolling log → prompt.
// Messages rendered as IRC-style log lines. No sidebar. No bubbles.

export default function TerminalTheme(props: ThemeProps) {
  const {
    peers, selfInfo, activePeer, activePeerData, messages, typing, unread, connected,
    themes, activeTheme, inputText, searchQuery, showGifPicker, gifQuery, gifResults,
    gifLoading, view, onSelectPeer, onSend, onInputChange, onInputKeyDown, onSearchChange,
    onReaction, onOpenGifs, onCloseGifs, onSearchGifs, onPickGif, onSetView, onSetTheme,
    messagesEndRef, textareaRef,
    // Group chat props (not yet rendered in terminal)
    activeGroup: _activeGroup, activeChat: _activeChat, groups: _groups, groupInvites: _groupInvites,
    onSelectGroup: _onSelectGroup, onCreateGroup: _onCreateGroup,
    onAcceptGroupInvite: _onAcceptGroupInvite, onDeclineGroupInvite: _onDeclineGroupInvite,
  } = props;

  const [showPeers, setShowPeers] = useState(true);

  const filtered = peers.filter(p => !p.IsSelf);
  const online = filtered.filter(p => p.RunningTailchat);
  const offline = filtered.filter(p => !p.RunningTailchat);

  return (
    <div className="crt-root crt-screen flex flex-col h-screen w-screen">
      {/* CRT effects */}
      <div className="crt-scanlines" />
      <div className="crt-vignette" />
      <div className="crt-glow" />

      {/* ── Status bar ── */}
      <div className="crt-statusbar drag-region shrink-0 px-4 pt-8 pb-1.5 flex items-center justify-between">
        <div className="crt-text flex items-center gap-4 text-[16px]">
          <span className="crt-text-bright">tailchat</span>
          <span className="crt-text-dim">|</span>
          <span className="crt-text-dim">{selfInfo.hostname}@{selfInfo.ip}</span>
          {activePeer && (
            <>
              <span className="crt-text-dim">|</span>
              <span>#{activePeer}</span>
              {connected[activePeer] && <span className="crt-text-dim">[e2e]</span>}
              {typing[activePeer] && <span className="text-[#ffaa00]" style={{ textShadow: '0 0 4px rgba(255,170,0,0.4)' }}>[typing...]</span>}
            </>
          )}
        </div>
        <div className="flex items-center gap-3 text-[16px]">
          <button
            onClick={() => setShowPeers(!showPeers)}
            className={`cursor-pointer transition-colors ${showPeers ? 'crt-text-bright' : 'crt-text-dim'} hover:text-[#33ff00]`}
          >
            [peers]
          </button>
          <button
            onClick={() => onSetView(view === 'settings' ? 'chat' : 'settings')}
            className={`cursor-pointer transition-colors ${view === 'settings' ? 'crt-text-bright' : 'crt-text-dim'} hover:text-[#33ff00]`}
          >
            [{view === 'settings' ? 'close' : 'config'}]
          </button>
        </div>
      </div>

      {/* ── Peer bar (toggleable) ── */}
      {showPeers && (
        <div className="crt-peerbar shrink-0 px-4 py-1.5 flex items-center gap-1 overflow-x-auto text-[15px]">
          <span className="crt-text-dim shrink-0">who:</span>
          {online.map(p => (
            <button key={p.Hostname} onClick={() => { if (view === 'settings') onSetView('chat'); onSelectPeer(p); }}
              className={`crt-peer-row shrink-0 px-2 py-0.5 cursor-pointer text-[15px] ${p.Hostname === activePeer ? 'crt-text-bright crt-peer-row-active' : 'crt-text'}`}>
              @{p.Hostname}{p.RunningTailchat ? '*' : ''}{unread[p.Hostname] ? `(${unread[p.Hostname]})` : ''}
            </button>
          ))}
          {offline.map(p => (
            <button key={p.Hostname} onClick={() => { if (view === 'settings') onSetView('chat'); onSelectPeer(p); }}
              className={`crt-peer-row shrink-0 px-2 py-0.5 cursor-pointer text-[15px] ${p.Hostname === activePeer ? 'crt-peer-row-active crt-text-dim' : 'crt-text-dim'}`}>
              @{p.Hostname}
            </button>
          ))}
          {filtered.length === 0 && <span className="crt-text-dim italic">no peers on tailnet</span>}
        </div>
      )}

      {/* ── Main content ── */}
      {view === 'settings' ? (
        <TerminalSettings themes={themes} activeTheme={activeTheme} onSetTheme={onSetTheme} />
      ) : !activePeer ? (
        <TerminalEmptyState />
      ) : (
        <>
          {/* Message log */}
          <div className="flex-1 overflow-y-auto px-4 py-2 text-[16px]">
            {messages.length === 0 && (
              <div className="crt-text-dim py-4">
                <p>*** Encrypted channel #{activePeer} joined</p>
                <p>*** Cipher: X25519 + XChaCha20-Poly1305</p>
                <p>*** Waiting for messages...</p>
              </div>
            )}
            {messages.map(msg => (
              <TerminalLogLine key={msg.ID} msg={msg} onReaction={onReaction} />
            ))}
            <div ref={messagesEndRef} />
          </div>

          {/* Input prompt */}
          <div className="shrink-0 px-4 py-2 border-t border-[#1a3300]">
            <div className="flex items-center gap-0 text-[16px]">
              <button onClick={onOpenGifs} className="crt-text-dim hover:text-[#33ff00] cursor-pointer mr-2 transition-colors">[gif]</button>
              <span className="crt-text-bright mr-1">{'>'}</span>
              <textarea
                ref={textareaRef}
                placeholder="type a message..."
                value={inputText}
                onChange={onInputChange}
                onKeyDown={onInputKeyDown}
                rows={1}
                className="crt-input flex-1 text-[16px] min-h-[24px] max-h-24 leading-normal resize-none bg-transparent"
              />
              <span className="crt-cursor crt-text-bright">█</span>
              {inputText.trim() && (
                <button onClick={onSend} className="crt-text-bright hover:text-[#66ff33] cursor-pointer ml-2 transition-colors">[send]</button>
              )}
            </div>
          </div>
        </>
      )}

      {/* GIF picker */}
      {showGifPicker && (
        <div className="fixed inset-0 bg-[rgba(0,0,0,0.7)] z-50 flex items-end justify-center" onClick={onCloseGifs}>
          <div className="bg-[#0d0d0d] border border-[#1a3300] w-[500px] max-h-[350px] flex flex-col rounded-t-lg" onClick={e => e.stopPropagation()}>
            <div className="p-2 border-b border-[#1a3300] shrink-0 flex items-center gap-2 text-[16px]">
              <span className="crt-text-dim">search:</span>
              <input type="text" value={gifQuery} onChange={e => onSearchGifs(e.target.value)} autoFocus
                className="crt-input flex-1 text-[16px] bg-transparent" placeholder="query..." />
            </div>
            <div className="flex-1 overflow-y-auto p-2 grid grid-cols-3 gap-1">
              {gifLoading && <p className="col-span-3 text-center py-6 crt-text-dim text-[15px]">loading...</p>}
              {!gifLoading && gifResults.map(gif => (
                <div key={gif.ID} onClick={() => onPickGif(gif)} className="aspect-square overflow-hidden cursor-pointer border border-[#1a3300] hover:border-[#33ff00] transition-colors">
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

function TerminalLogLine({ msg, onReaction }: { msg: Message; onReaction: (id: string, emoji: string) => void }) {
  const words = msg.Content.trim().split(/\s+/);
  const isGif = words.length === 1 && isImageURL(words[0]);
  const reactions = new Map<string, string[]>();
  if (msg.Reactions) for (const r of msg.Reactions) reactions.set(r.Emoji, [...(reactions.get(r.Emoji) || []), r.Sender]);

  const time = formatTime(msg.Timestamp);
  const sender = msg.IsOwn ? 'you' : msg.Sender;
  const status = msg.IsOwn ? (msg.State === 2 ? ' ✓✓' : msg.State === 1 ? ' ✓' : ' ...') : '';

  return (
    <div className="group/msg crt-log-line relative leading-relaxed py-[1px]">
      <span className="crt-text-dim">[{time}]</span>{' '}
      <span className={msg.IsOwn ? 'crt-text-bright' : 'crt-text'}>&lt;{sender}&gt;</span>{' '}
      {isGif ? (
        <span>
          <span className="crt-text-dim">[image: </span>
          <img src={words[0]} alt="GIF" className="inline-block max-w-[200px] max-h-[150px] align-middle border border-[#1a3300] my-1" />
          <span className="crt-text-dim">]</span>
        </span>
      ) : (
        <span className="crt-text">{msg.Content}</span>
      )}
      <span className="crt-text-dim text-[14px]">{status}</span>

      {/* Inline reactions */}
      {reactions.size > 0 && (
        <span className="ml-2">
          {Array.from(reactions.entries()).map(([emoji, senders]) => (
            <button key={emoji} onClick={() => onReaction(msg.ID, emoji)} title={senders.join(', ')}
              className="crt-reaction inline-flex items-center gap-0.5 px-1 py-0 text-[14px] mx-0.5">
              {emoji}<span className="crt-text-dim text-[13px]">{senders.length}</span>
            </button>
          ))}
        </span>
      )}

      {/* Hover reaction bar */}
      <span className="crt-msg-actions absolute right-0 top-0 bg-[#0d0d0d] border border-[#1a3300] px-0.5 flex items-center gap-0">
        {QUICK_REACTIONS.map(emoji => (
          <button key={emoji} onClick={() => onReaction(msg.ID, emoji)}
            className="w-5 h-5 flex items-center justify-center text-[13px] hover:bg-[rgba(51,255,0,0.06)] cursor-pointer transition-colors">
            {emoji}
          </button>
        ))}
      </span>
    </div>
  );
}

function TerminalEmptyState() {
  return (
    <div className="flex-1 overflow-y-auto px-4 py-4 text-[16px]">
      <p className="crt-text-dim">*** tailchat v1.0 — encrypted chat over tailscale</p>
      <p className="crt-text-dim">*** cipher: X25519 key exchange + XChaCha20-Poly1305</p>
      <p className="crt-text-dim">*** zero trust architecture — no central server</p>
      <p className="crt-text-dim mt-2">*** select a peer from the list above to begin</p>
      <p className="crt-text mt-4">
        <span className="crt-text-bright">{'>'}</span> _<span className="crt-cursor">█</span>
      </p>
    </div>
  );
}

function TerminalSettings({ themes, activeTheme, onSetTheme }: { themes: ThemeInfo[]; activeTheme: string; onSetTheme: (name: string) => void }) {
  return (
    <div className="crt-settings flex-1 overflow-y-auto px-4 py-4 text-[16px]">
      <p className="crt-text-bright mb-1">*** CONFIGURATION</p>
      <p className="crt-text-dim mb-4">*** system preferences and theme selection</p>

      <p className="crt-text mb-2">--- THEMES ---</p>
      <p className="crt-text-dim mb-3">custom themes: ~/.tailchat/themes/{'<name>'}/ </p>

      <div className="space-y-1 mb-6 max-w-[500px]">
        {themes.map((t, i) => (
          <div key={t.name} onClick={() => onSetTheme(t.name)}
            className={`crt-theme-row px-3 py-2 flex items-center gap-3 ${t.name === activeTheme ? 'crt-theme-row-active' : ''}`}>
            <span className="crt-text-dim w-4">{i + 1}.</span>
            <span className={t.name === activeTheme ? 'crt-text-bright' : 'crt-text'}>{t.name}</span>
            <span className="crt-text-dim flex-1 truncate">— {t.description}</span>
            {t.name === activeTheme && <span className="crt-text-bright">[*]</span>}
          </div>
        ))}
      </div>

      <p className="crt-text mb-2">--- CREATE A THEME ---</p>
      <div className="crt-text-dim leading-relaxed max-w-[500px]">
        <p>  mkdir ~/.tailchat/themes/my-theme/</p>
        <p>  touch ~/.tailchat/themes/my-theme/index.html</p>
        <p className="mt-2">  # Your theme gets full access to:</p>
        <p>  #   window.go.main.App.*  (Go backend)</p>
        <p>  #   window.runtime.*      (Wails runtime)</p>
      </div>
    </div>
  );
}
