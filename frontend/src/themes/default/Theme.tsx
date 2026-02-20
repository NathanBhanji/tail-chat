import { useState } from 'react';
import type { ThemeProps, Message, Peer, Group, GroupInvite, ThemeInfo } from '../types';
import './default.css';

// ─── Helpers ────────────────────────────────────────────────────────

const isImageURL = (s: string) => {
  const u = s.trim().toLowerCase();
  return u.includes('tenor.com') || u.includes('giphy.com') || /\.(gif|png|jpg|jpeg|webp)(\?.*)?$/.test(u);
};

const formatTime = (ts: string) =>
  new Date(ts).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });

const QUICK_REACTIONS = ['👍', '😂', '🔥', '❤️', '👀'];

// Deterministic gradient from string — unique identity for peers and groups
function hashGradient(name: string): string {
  let hash = 0;
  for (let i = 0; i < name.length; i++) hash = ((hash << 5) - hash + name.charCodeAt(i)) | 0;
  const h1 = ((hash & 0xff) / 255) * 360;
  const h2 = (h1 + 40 + ((hash >> 8) & 0x3f)) % 360;
  return `linear-gradient(135deg, oklch(0.55 0.12 ${h1}), oklch(0.45 0.10 ${h2}))`;
}

function initials(h: string): string {
  return h.slice(0, 2).toUpperCase();
}

// ─── Icons ──────────────────────────────────────────────────────────

const IconSearch = () => (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
    <circle cx="11" cy="11" r="8"/><path d="m21 21-4.35-4.35"/>
  </svg>
);

const IconShield = () => (
  <svg width="11" height="11" viewBox="0 0 24 24" fill="currentColor">
    <path d="M12 2L3 7v5c0 5.55 3.84 10.74 9 12 5.16-1.26 9-6.45 9-12V7l-9-5z"/>
  </svg>
);

const IconSettings = () => (
  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
    <circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>
  </svg>
);

const IconSend = () => (
  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
    <path d="m5 12 14 0M13 5l6 7-6 7"/>
  </svg>
);

const IconLock = () => (
  <svg width="10" height="10" viewBox="0 0 24 24" fill="currentColor">
    <path d="M18 8h-1V6c0-2.76-2.24-5-5-5S7 3.24 7 6v2H6c-1.1 0-2 .9-2 2v10c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V10c0-1.1-.9-2-2-2zM12 17c-1.1 0-2-.9-2-2s.9-2 2-2 2 .9 2 2-.9 2-2 2zM9 8V6c0-1.66 1.34-3 3-3s3 1.34 3 3v2H9z"/>
  </svg>
);

const IconX = () => (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round">
    <path d="M18 6L6 18M6 6l12 12"/>
  </svg>
);

const IconPlus = () => (
  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round">
    <path d="M12 5v14M5 12h14"/>
  </svg>
);

const IconUsers = () => (
  <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
    <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/>
  </svg>
);

const IconCheck = () => (
  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round">
    <path d="M20 6L9 17l-5-5"/>
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
    peers, selfInfo, activePeer, activePeerData, activeGroup, activeChat,
    messages, typing, unread, connected, groups, groupInvites,
    themes, activeTheme, inputText, searchQuery, showGifPicker, gifQuery, gifResults,
    gifLoading, view, onSelectPeer, onSelectGroup, onSend, onInputChange, onInputKeyDown,
    onSearchChange, onReaction, onOpenGifs, onCloseGifs, onSearchGifs, onPickGif,
    onSetView, onSetTheme, onCreateGroup, onAcceptGroupInvite, onDeclineGroupInvite,
    messagesEndRef, textareaRef,
  } = props;

  const [showCreateGroup, setShowCreateGroup] = useState(false);
  const [newGroupName, setNewGroupName] = useState('');
  const [selectedMembers, setSelectedMembers] = useState<string[]>([]);

  const filtered = peers.filter(p => !p.IsSelf && p.Hostname.toLowerCase().includes(searchQuery.toLowerCase()));
  const online = filtered.filter(p => p.RunningTailchat);
  const offline = filtered.filter(p => !p.RunningTailchat);

  const filteredGroups = groups.filter(g => g.Name.toLowerCase().includes(searchQuery.toLowerCase()));

  // Who is the active conversation target?
  const chatLabel = activeGroup ? activeGroup.Name : activePeer;
  const hasActiveChat = !!(activePeer || activeGroup);

  return (
    <div className="flex h-screen w-screen" style={{ background: '#0a0a0f', fontFamily: 'var(--font-sans)' }}>
      {/* ── Sidebar ── */}
      <aside className="cl-sidebar relative w-[264px] min-w-[264px] flex flex-col h-full">
        {/* Brand */}
        <div className="drag-region cl-brand pt-11 px-5 pb-4 shrink-0">
          <div className="flex items-center gap-2">
            <div className="w-7 h-7 rounded-lg flex items-center justify-center" style={{ background: 'linear-gradient(135deg, rgba(57,255,159,0.15), rgba(108,180,255,0.1))' }}>
              <IconLock />
            </div>
            <h1 className="cl-brand-text text-[15px] font-bold tracking-tight">tailchat</h1>
          </div>
          <div className="flex items-center gap-1.5 mt-2 ml-0.5">
            <span className="cl-status-tailchat w-[5px] h-[5px] rounded-full shrink-0" />
            <span className="font-mono text-[10.5px] text-[rgba(232,232,240,0.2)]">{selfInfo.hostname}</span>
            <span className="text-[rgba(232,232,240,0.08)]">/</span>
            <span className="font-mono text-[10.5px] text-[rgba(232,232,240,0.12)]">{selfInfo.ip}</span>
          </div>
        </div>

        {/* Search */}
        <div className="px-4 pb-2 shrink-0 relative z-10">
          <div className="relative">
            <div className="absolute left-2.5 top-1/2 -translate-y-1/2 text-[rgba(232,232,240,0.15)]"><IconSearch /></div>
            <input type="text" placeholder="Search..." value={searchQuery}
              onChange={e => onSearchChange(e.target.value)}
              className="cl-search w-full rounded-lg pl-8 pr-3 py-[7px] text-[12.5px]" />
          </div>
        </div>

        {/* Scrollable list */}
        <div className="flex-1 overflow-y-auto px-3 py-1 relative z-10">
          {/* Group invites */}
          {groupInvites.length > 0 && (
            <>
              <div className="cl-section-label text-[#e8b931]">Invites — {groupInvites.length}</div>
              {groupInvites.map(inv => (
                <CipherInviteItem key={inv.groupID} invite={inv} onAccept={onAcceptGroupInvite} onDecline={onDeclineGroupInvite} />
              ))}
            </>
          )}

          {/* Groups */}
          {filteredGroups.length > 0 && (
            <>
              <div className="cl-section-label flex items-center justify-between">
                <span>Groups — {filteredGroups.length}</span>
                <button onClick={() => setShowCreateGroup(true)} className="text-[rgba(232,232,240,0.2)] hover:text-[#39ff9f] transition-colors cursor-pointer p-0.5" title="Create group">
                  <IconPlus />
                </button>
              </div>
              {filteredGroups.map(g => {
                const chatKey = `group:${g.ID}`;
                return (
                  <CipherGroupItem key={g.ID} group={g} active={activeChat === chatKey} unread={unread[chatKey] || 0}
                    onClick={() => { if (view === 'settings') onSetView('chat'); onSelectGroup(g); }} />
                );
              })}
            </>
          )}

          {/* Online peers */}
          {online.length > 0 && (
            <>
              <div className="cl-section-label flex items-center justify-between">
                <span>Online — {online.length}</span>
                {filteredGroups.length === 0 && (
                  <button onClick={() => setShowCreateGroup(true)} className="text-[rgba(232,232,240,0.2)] hover:text-[#39ff9f] transition-colors cursor-pointer p-0.5" title="Create group">
                    <IconPlus />
                  </button>
                )}
              </div>
              {online.map(p => <CipherPeerItem key={p.Hostname} peer={p} active={p.Hostname === activePeer} unread={unread[p.Hostname] || 0} connected={connected[p.Hostname] || false} onClick={() => { if (view === 'settings') onSetView('chat'); onSelectPeer(p); }} />)}
            </>
          )}
          {offline.length > 0 && (
            <>
              <div className="cl-section-label">Offline — {offline.length}</div>
              {offline.map(p => <CipherPeerItem key={p.Hostname} peer={p} active={p.Hostname === activePeer} unread={0} connected={false} onClick={() => { if (view === 'settings') onSetView('chat'); onSelectPeer(p); }} />)}
            </>
          )}
          {filtered.length === 0 && filteredGroups.length === 0 && <p className="px-2 py-8 text-[rgba(232,232,240,0.15)] text-[11.5px] text-center">No results</p>}
        </div>

        {/* Settings */}
        <div className="px-3 py-2.5 shrink-0 relative z-10">
          <button onClick={() => onSetView(view === 'settings' ? 'chat' : 'settings')}
            className={`cl-settings-btn w-full flex items-center gap-2.5 px-3 py-2 rounded-lg text-[12.5px] font-medium
              ${view === 'settings' ? 'cl-settings-btn-active' : 'text-[rgba(232,232,240,0.3)]'}`}>
            {view === 'settings' ? <IconX /> : <IconSettings />}
            {view === 'settings' ? 'Close' : 'Settings'}
          </button>
        </div>
      </aside>

      {/* ── Main ── */}
      <main className="flex-1 flex flex-col h-full min-w-0">
        {view === 'settings' ? (
          <div className="view-enter flex flex-col h-full">
            <CipherSettingsView themes={themes} activeTheme={activeTheme} onSetTheme={onSetTheme} />
          </div>
        ) : !hasActiveChat ? (
          <div className="view-enter flex flex-col h-full">
            <CipherEmptyState />
          </div>
        ) : (
          <div className="view-enter cl-chat-area flex flex-col h-full">
            {/* Header — adapts for DM vs group */}
            <header className="drag-region cl-header pt-11 px-6 pb-3 shrink-0">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <div className="cl-avatar w-8 h-8 rounded-[10px] flex items-center justify-center text-[11px] font-bold text-white/80"
                    style={{ background: hashGradient(chatLabel) }}>
                    {activeGroup ? <IconUsers /> : initials(activePeer)}
                  </div>
                  <div>
                    <h2 className="text-[14.5px] font-semibold text-[#e8e8f0] tracking-tight">{chatLabel}</h2>
                    <div className="flex items-center gap-1.5 mt-0.5">
                      <span className="cl-badge cl-badge-e2e"><IconLock /> E2E</span>
                      {activeGroup && (
                        <span className="cl-badge cl-badge-os">
                          <IconUsers /> {activeGroup.Members.length} members
                        </span>
                      )}
                      {activePeer && connected[activePeer] && (
                        <span className="cl-badge cl-badge-connected">
                          <span className="w-1.5 h-1.5 rounded-full bg-[#39ff9f]" /> Connected
                        </span>
                      )}
                      {activePeer && typing[activePeer] && <span className="text-[11px] text-[#6cb4ff] italic animate-pulse ml-1">typing...</span>}
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-1.5">
                  {activePeerData && !activeGroup && (
                    <>
                      <span className="cl-badge cl-badge-os">{activePeerData.OS}</span>
                      {activePeerData.RunningTailchat && <span className="cl-badge cl-badge-e2e"><IconShield /> tailchat</span>}
                    </>
                  )}
                  {activeGroup && (
                    <span className="cl-badge cl-badge-os font-mono text-[9px]">
                      {activeGroup.Members.filter(m => m !== selfInfo.hostname).join(', ')}
                    </span>
                  )}
                </div>
              </div>
            </header>

            {/* Messages */}
            <div className="flex-1 overflow-y-auto px-6 py-4">
              {messages.length === 0 && (
                <div className="flex items-center justify-center h-full">
                  <div className="text-center" style={{ animation: 'cl-fade-up 0.3s ease-out' }}>
                    <div className="w-10 h-10 rounded-xl bg-[rgba(255,255,255,0.03)] border border-[rgba(255,255,255,0.04)] flex items-center justify-center mx-auto mb-3">
                      <span className="text-[#39ff9f] opacity-40">{activeGroup ? <IconUsers /> : <IconLock />}</span>
                    </div>
                    <p className="text-[rgba(232,232,240,0.4)] text-[12.5px] font-medium">
                      {activeGroup ? 'No group messages yet' : 'Encrypted channel open'}
                    </p>
                    <p className="text-[rgba(232,232,240,0.15)] text-[11px] mt-0.5">Send a message to begin</p>
                  </div>
                </div>
              )}
              {groupMessages(messages).map((g, i) => (
                <CipherMessageGroup key={i} group={g} onReaction={onReaction} />
              ))}
              <div ref={messagesEndRef} />
            </div>

            {/* Input */}
            <div className="cl-input-panel px-6 py-3 shrink-0">
              <div className="flex items-end gap-2.5">
                <button onClick={onOpenGifs} className="cl-gif-btn shrink-0 w-9 h-9 rounded-lg flex items-center justify-center cursor-pointer">
                  <span className="font-mono text-[10px] font-bold tracking-wider">GIF</span>
                </button>
                <textarea ref={textareaRef} placeholder={`Message ${chatLabel}...`} value={inputText}
                  onChange={onInputChange} onKeyDown={onInputKeyDown} rows={1}
                  className="cl-textarea flex-1 rounded-xl px-4 py-2.5 text-[13.5px] min-h-[40px] max-h-40 leading-relaxed resize-none" />
                <button onClick={onSend} disabled={!inputText.trim()}
                  className="cl-send-btn shrink-0 w-9 h-9 rounded-full flex items-center justify-center cursor-pointer">
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
          <div className="cl-gif-panel w-[480px] max-h-[440px] rounded-t-2xl flex flex-col shadow-2xl" style={{ animation: 'slide-up 0.2s ease-out' }} onClick={e => e.stopPropagation()}>
            <div className="p-3 border-b border-[rgba(255,255,255,0.04)] shrink-0">
              <div className="relative">
                <div className="absolute left-2.5 top-1/2 -translate-y-1/2 text-[rgba(232,232,240,0.15)]"><IconSearch /></div>
                <input type="text" placeholder="Search GIFs..." value={gifQuery} onChange={e => onSearchGifs(e.target.value)} autoFocus
                  className="cl-search w-full rounded-lg pl-8 pr-3 py-2 text-[12.5px]" />
              </div>
            </div>
            <div className="flex-1 overflow-y-auto p-2 grid grid-cols-3 gap-1.5">
              {gifLoading && <p className="col-span-3 text-center py-8 text-[rgba(232,232,240,0.15)] text-[11.5px]">Loading...</p>}
              {!gifLoading && gifResults.map(gif => (
                <div key={gif.ID} onClick={() => onPickGif(gif)} className="aspect-square rounded-lg overflow-hidden cursor-pointer bg-[rgba(255,255,255,0.02)] hover:scale-[1.03] transition-transform duration-150 border border-[rgba(255,255,255,0.03)] hover:border-[rgba(57,255,159,0.1)]">
                  <img src={gif.Media.TinyGIF?.URL || gif.Media.NanoGIF?.URL || gif.Media.GIF?.URL} alt={gif.Title} loading="lazy" className="w-full h-full object-cover" />
                </div>
              ))}
            </div>
          </div>
        </div>
      )}

      {/* Create Group Modal */}
      {showCreateGroup && (
        <CipherCreateGroupModal
          peers={peers.filter(p => !p.IsSelf && p.RunningTailchat)}
          selected={selectedMembers}
          name={newGroupName}
          onNameChange={setNewGroupName}
          onToggleMember={(h) => setSelectedMembers(prev => prev.includes(h) ? prev.filter(x => x !== h) : [...prev, h])}
          onCreate={() => {
            if (newGroupName.trim() && selectedMembers.length > 0) {
              onCreateGroup(newGroupName.trim(), selectedMembers);
              setShowCreateGroup(false);
              setNewGroupName('');
              setSelectedMembers([]);
            }
          }}
          onClose={() => { setShowCreateGroup(false); setNewGroupName(''); setSelectedMembers([]); }}
        />
      )}
    </div>
  );
}

// ─── Sub-components ─────────────────────────────────────────────────

function CipherPeerItem({ peer, active, unread, connected, onClick }: { peer: Peer; active: boolean; unread: number; connected: boolean; onClick: () => void }) {
  const isOnline = peer.RunningTailchat && peer.Online;
  return (
    <div onClick={onClick} className={`cl-peer flex items-center gap-2.5 px-2.5 py-[9px] rounded-lg cursor-pointer ${active ? 'cl-peer-active' : ''}`}>
      <div className="relative shrink-0">
        <div className="cl-avatar w-[34px] h-[34px] flex items-center justify-center font-mono text-[11px] font-bold"
          style={{
            background: active || isOnline ? hashGradient(peer.Hostname) : 'linear-gradient(135deg, #1a1a22, #141418)',
            color: active || isOnline ? 'rgba(255,255,255,0.85)' : 'rgba(232,232,240,0.25)',
            borderRadius: '10px',
          }}>
          {initials(peer.Hostname)}
        </div>
        <span className={`absolute -bottom-0.5 -right-0.5 w-[10px] h-[10px] rounded-full border-[2px]
          ${active ? 'border-[rgba(57,255,159,0.06)]' : 'border-[#111116]'}
          ${isOnline ? 'cl-status-tailchat' : 'cl-status-offline'}`} />
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-1.5">
          <span className={`text-[12.5px] font-medium truncate ${active ? 'text-[#e8e8f0]' : 'text-[rgba(232,232,240,0.7)]'}`}>{peer.Hostname}</span>
          {peer.RunningTailchat && <span className="text-[rgba(57,255,159,0.5)] shrink-0"><IconShield /></span>}
        </div>
        <div className={`text-[10.5px] font-mono truncate ${active ? 'text-[rgba(232,232,240,0.3)]' : 'text-[rgba(232,232,240,0.15)]'}`}>
          {peer.OS}{connected ? ' · connected' : ''}
        </div>
      </div>
      {unread > 0 && <span className="cl-unread min-w-[18px] h-[18px] rounded-full text-[9.5px] flex items-center justify-center px-1">{unread}</span>}
    </div>
  );
}

function CipherGroupItem({ group, active, unread, onClick }: { group: Group; active: boolean; unread: number; onClick: () => void }) {
  return (
    <div onClick={onClick} className={`cl-peer flex items-center gap-2.5 px-2.5 py-[9px] rounded-lg cursor-pointer ${active ? 'cl-peer-active' : ''}`}>
      <div className="cl-avatar w-[34px] h-[34px] rounded-[10px] flex items-center justify-center"
        style={{ background: active ? hashGradient(group.Name) : 'linear-gradient(135deg, rgba(108,180,255,0.12), rgba(57,255,159,0.08))', color: active ? 'rgba(255,255,255,0.85)' : 'rgba(108,180,255,0.5)' }}>
        <IconUsers />
      </div>
      <div className="flex-1 min-w-0">
        <span className={`text-[12.5px] font-medium truncate block ${active ? 'text-[#e8e8f0]' : 'text-[rgba(232,232,240,0.7)]'}`}>{group.Name}</span>
        <span className={`text-[10.5px] font-mono truncate block ${active ? 'text-[rgba(232,232,240,0.3)]' : 'text-[rgba(232,232,240,0.15)]'}`}>
          {group.Members.length} members
        </span>
      </div>
      {unread > 0 && <span className="cl-unread min-w-[18px] h-[18px] rounded-full text-[9.5px] flex items-center justify-center px-1">{unread}</span>}
    </div>
  );
}

function CipherInviteItem({ invite, onAccept, onDecline }: { invite: GroupInvite; onAccept: (inv: GroupInvite) => void; onDecline: (inv: GroupInvite) => void }) {
  return (
    <div className="cl-peer flex items-center gap-2.5 px-2.5 py-[9px] rounded-lg" style={{ background: 'rgba(232,185,49,0.04)', borderLeft: '2px solid rgba(232,185,49,0.3)' }}>
      <div className="cl-avatar w-[34px] h-[34px] rounded-[10px] flex items-center justify-center" style={{ background: 'linear-gradient(135deg, rgba(232,185,49,0.15), rgba(232,185,49,0.08))', color: '#e8b931' }}>
        <IconUsers />
      </div>
      <div className="flex-1 min-w-0">
        <span className="text-[12.5px] font-medium truncate block text-[#e8b931]">{invite.groupName}</span>
        <span className="text-[10px] font-mono text-[rgba(232,232,240,0.2)] truncate block">from {invite.from}</span>
      </div>
      <div className="flex items-center gap-1 shrink-0">
        <button onClick={() => onAccept(invite)} className="w-6 h-6 rounded-md bg-[rgba(57,255,159,0.1)] text-[#39ff9f] flex items-center justify-center hover:bg-[rgba(57,255,159,0.2)] transition-colors cursor-pointer" title="Accept">
          <IconCheck />
        </button>
        <button onClick={() => onDecline(invite)} className="w-6 h-6 rounded-md bg-[rgba(255,107,107,0.1)] text-[#ff6b6b] flex items-center justify-center hover:bg-[rgba(255,107,107,0.2)] transition-colors cursor-pointer" title="Decline">
          <IconX />
        </button>
      </div>
    </div>
  );
}

function CipherMessageGroup({ group, onReaction }: { group: MsgGroup; onReaction: (id: string, emoji: string) => void }) {
  return (
    <div className={`cl-msg-group msg-row ${group.isOwn ? 'cl-msg-group-own' : 'cl-msg-group-other'}`}>
      <div className="flex items-center gap-2 mb-1">
        <span className={`text-[12.5px] font-semibold ${group.isOwn ? 'text-[#39ff9f]' : 'text-[#6cb4ff]'}`}>{group.sender}</span>
        <span className="text-[10px] font-mono text-[rgba(232,232,240,0.15)]">{group.time}</span>
      </div>
      {group.messages.map(msg => <CipherMessageRow key={msg.ID} msg={msg} onReaction={onReaction} />)}
    </div>
  );
}

function CipherMessageRow({ msg, onReaction }: { msg: Message; onReaction: (id: string, emoji: string) => void }) {
  const words = msg.Content.trim().split(/\s+/);
  const isGif = words.length === 1 && isImageURL(words[0]);
  const reactions = new Map<string, string[]>();
  if (msg.Reactions) for (const r of msg.Reactions) reactions.set(r.Emoji, [...(reactions.get(r.Emoji) || []), r.Sender]);

  return (
    <div className="group/msg cl-msg-row relative flex items-start gap-2">
      <div className="flex-1 min-w-0">
        {isGif ? (
          <div className="rounded-lg overflow-hidden max-w-[280px] my-1 border border-[rgba(255,255,255,0.04)] hover:border-[rgba(255,255,255,0.08)] transition-colors duration-150">
            <img src={words[0]} alt="GIF" className="block w-full h-auto" />
          </div>
        ) : (
          <p className="text-[13px] text-[rgba(232,232,240,0.85)] leading-[1.6] break-words">{msg.Content}</p>
        )}
        {msg.IsOwn && (
          <span className={`font-mono text-[9.5px] ${msg.State === 2 ? 'cl-state-read' : msg.State === 1 ? 'cl-state-delivered' : 'cl-state-sending'}`}>
            {msg.State === 2 ? 'Read' : msg.State === 1 ? 'Delivered' : 'Sending...'}
          </span>
        )}
        {reactions.size > 0 && (
          <div className="flex gap-1 mt-1 flex-wrap">
            {Array.from(reactions.entries()).map(([emoji, senders]) => (
              <button key={emoji} onClick={() => onReaction(msg.ID, emoji)} title={senders.join(', ')} className="cl-reaction">
                {emoji} <span className="font-mono text-[9.5px] text-[rgba(232,232,240,0.25)]">{senders.length}</span>
              </button>
            ))}
          </div>
        )}
      </div>
      <div className="cl-reactions-bar absolute -top-3 right-0 flex items-center gap-0.5 bg-[rgba(20,20,28,0.95)] border border-[rgba(255,255,255,0.06)] rounded-lg shadow-lg px-0.5 py-0.5 backdrop-blur-sm">
        {QUICK_REACTIONS.map(emoji => (
          <button key={emoji} onClick={() => onReaction(msg.ID, emoji)}
            className="w-6 h-6 flex items-center justify-center rounded text-[12px] hover:bg-[rgba(255,255,255,0.04)] transition-colors cursor-pointer" title={`React with ${emoji}`}>
            {emoji}
          </button>
        ))}
      </div>
    </div>
  );
}

function CipherEmptyState() {
  return (
    <>
      <header className="drag-region cl-header pt-11 px-6 pb-3 shrink-0" />
      <div className="flex-1 flex flex-col items-center justify-center text-center px-12" style={{ animation: 'cl-fade-up 0.4s ease-out' }}>
        <div className="cl-empty-icon relative mb-8">
          <div className="w-[72px] h-[72px] rounded-2xl bg-[rgba(255,255,255,0.02)] border border-[rgba(255,255,255,0.04)] flex items-center justify-center">
            <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" className="text-[#39ff9f] opacity-40">
              <path d="M12 2L3 7v5c0 5.55 3.84 10.74 9 12 5.16-1.26 9-6.45 9-12V7l-9-5z"/>
              <path d="M12 11v2M12 16h.01" strokeWidth="2"/>
            </svg>
          </div>
        </div>
        <h3 className="text-[15px] font-semibold text-[#e8e8f0] mb-1.5 tracking-tight">Select a peer or group</h3>
        <p className="text-[11.5px] text-[rgba(232,232,240,0.2)] max-w-[300px] leading-relaxed">
          End-to-end encrypted with X25519 key exchange and XChaCha20-Poly1305 authenticated encryption
        </p>
        <div className="mt-6 flex items-center gap-3">
          <span className="w-10 h-px bg-gradient-to-r from-transparent to-[rgba(57,255,159,0.1)]" />
          <span className="font-mono text-[9px] uppercase tracking-[0.2em] text-[rgba(232,232,240,0.1)]">Zero Trust</span>
          <span className="w-10 h-px bg-gradient-to-l from-transparent to-[rgba(108,180,255,0.1)]" />
        </div>
      </div>
    </>
  );
}

function CipherCreateGroupModal({ peers, selected, name, onNameChange, onToggleMember, onCreate, onClose }: {
  peers: Peer[]; selected: string[]; name: string; onNameChange: (n: string) => void; onToggleMember: (h: string) => void; onCreate: () => void; onClose: () => void;
}) {
  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center" onClick={onClose}>
      <div className="cl-gif-panel w-[400px] rounded-2xl flex flex-col shadow-2xl" style={{ animation: 'cl-fade-up 0.2s ease-out' }} onClick={e => e.stopPropagation()}>
        <div className="p-5 border-b border-[rgba(255,255,255,0.04)]">
          <h3 className="text-[14px] font-semibold text-[#e8e8f0] mb-1">Create Group</h3>
          <p className="text-[11px] text-[rgba(232,232,240,0.2)]">Name your group and select members</p>
        </div>
        <div className="p-5 space-y-4">
          <div>
            <label className="text-[10.5px] font-mono text-[rgba(232,232,240,0.25)] uppercase tracking-wider block mb-1.5">Group Name</label>
            <input type="text" placeholder="e.g. project-alpha" value={name} onChange={e => onNameChange(e.target.value)}
              className="cl-search w-full rounded-lg px-3 py-2 text-[12.5px]" autoFocus />
          </div>
          <div>
            <label className="text-[10.5px] font-mono text-[rgba(232,232,240,0.25)] uppercase tracking-wider block mb-1.5">Members ({selected.length} selected)</label>
            <div className="space-y-1 max-h-[200px] overflow-y-auto">
              {peers.map(p => (
                <div key={p.Hostname} onClick={() => onToggleMember(p.Hostname)}
                  className={`flex items-center gap-2.5 px-3 py-2 rounded-lg cursor-pointer transition-all duration-150
                    ${selected.includes(p.Hostname) ? 'bg-[rgba(57,255,159,0.06)] border border-[rgba(57,255,159,0.15)]' : 'bg-[rgba(255,255,255,0.02)] border border-transparent hover:bg-[rgba(255,255,255,0.03)]'}`}>
                  <div className="cl-avatar w-7 h-7 rounded-lg flex items-center justify-center font-mono text-[9px] font-bold text-white/80"
                    style={{ background: hashGradient(p.Hostname) }}>
                    {initials(p.Hostname)}
                  </div>
                  <span className="text-[12px] text-[rgba(232,232,240,0.7)] flex-1">{p.Hostname}</span>
                  {selected.includes(p.Hostname) && <span className="text-[#39ff9f]"><IconCheck /></span>}
                </div>
              ))}
              {peers.length === 0 && <p className="text-[11px] text-[rgba(232,232,240,0.15)] text-center py-4">No online peers with tailchat</p>}
            </div>
          </div>
        </div>
        <div className="p-4 border-t border-[rgba(255,255,255,0.04)] flex items-center justify-end gap-2">
          <button onClick={onClose} className="px-4 py-2 rounded-lg text-[12px] text-[rgba(232,232,240,0.3)] hover:text-[rgba(232,232,240,0.5)] transition-colors cursor-pointer">Cancel</button>
          <button onClick={onCreate} disabled={!name.trim() || selected.length === 0}
            className={`cl-send-btn px-5 py-2 rounded-lg text-[12px] font-medium flex items-center gap-2 cursor-pointer ${!name.trim() || selected.length === 0 ? 'opacity-30' : ''}`}>
            <IconPlus /> Create
          </button>
        </div>
      </div>
    </div>
  );
}

function CipherSettingsView({ themes, activeTheme, onSetTheme }: { themes: ThemeInfo[]; activeTheme: string; onSetTheme: (name: string) => void }) {
  return (
    <>
      <header className="drag-region cl-header pt-11 px-6 pb-3 shrink-0">
        <h2 className="text-[14.5px] font-semibold text-[#e8e8f0] tracking-tight">Settings</h2>
        <p className="text-[10.5px] text-[rgba(232,232,240,0.18)] mt-0.5 font-mono">Configuration & appearance</p>
      </header>
      <div className="flex-1 overflow-y-auto px-6 py-6" style={{ animation: 'cl-fade-up 0.2s ease-out' }}>
        <section>
          <h3 className="text-[12.5px] font-semibold text-[#e8e8f0] mb-1 tracking-tight">Themes</h3>
          <p className="text-[11.5px] text-[rgba(232,232,240,0.2)] mb-4 leading-relaxed">
            Choose a theme or install custom ones at <code className="font-mono text-[#39ff9f] text-[10.5px] bg-[rgba(57,255,159,0.06)] px-1.5 py-0.5 rounded border border-[rgba(57,255,159,0.08)]">~/.tailchat/themes/</code>
          </p>
          <div className="space-y-2">
            {themes.map(t => (
              <div key={t.name} onClick={() => onSetTheme(t.name)}
                className={`cl-theme-card flex items-center gap-4 p-4 rounded-xl ${t.name === activeTheme ? 'cl-theme-card-active' : ''}`}>
                <div className={`w-9 h-9 rounded-lg shrink-0 flex items-center justify-center text-[15px] ${t.name === activeTheme ? 'bg-[rgba(57,255,159,0.1)]' : 'bg-[rgba(255,255,255,0.03)]'}`}>
                  {t.name === 'default' ? '🌑' : t.name === 'aurora' ? '❄️' : t.name === 'vapor' ? '🌊' : t.name === 'retro-terminal' ? '📟' : '🎨'}
                </div>
                <div className="flex-1 min-w-0">
                  <div className={`text-[12.5px] font-medium ${t.name === activeTheme ? 'text-[#39ff9f]' : 'text-[#e8e8f0]'}`}>{t.name}</div>
                  {t.description && <div className="text-[10.5px] text-[rgba(232,232,240,0.2)] mt-0.5">{t.description}</div>}
                  {t.author && <div className="text-[9.5px] text-[rgba(232,232,240,0.12)] font-mono mt-0.5">by {t.author}</div>}
                </div>
                {t.name === activeTheme && <span className="font-mono text-[9px] font-semibold uppercase tracking-[0.15em] text-[#39ff9f] shrink-0">Active</span>}
              </div>
            ))}
          </div>
        </section>
        <section className="mt-8">
          <h3 className="text-[12.5px] font-semibold text-[#e8e8f0] mb-3 tracking-tight">Creating a theme</h3>
          <div className="bg-[rgba(255,255,255,0.02)] border border-[rgba(255,255,255,0.04)] rounded-xl p-5 text-[11.5px] text-[rgba(232,232,240,0.25)] leading-[1.7] space-y-2.5">
            <p>Create a folder in <code className="font-mono text-[#39ff9f] text-[10.5px] bg-[rgba(57,255,159,0.06)] px-1 py-0.5 rounded">~/.tailchat/themes/my-theme/</code> with an <code className="font-mono text-[#39ff9f] text-[10.5px] bg-[rgba(57,255,159,0.06)] px-1 py-0.5 rounded">index.html</code>.</p>
            <p>Your theme has full access to the Go backend via <code className="font-mono text-[#39ff9f] text-[10.5px] bg-[rgba(57,255,159,0.06)] px-1 py-0.5 rounded">window.go.main.App.*</code> and <code className="font-mono text-[#39ff9f] text-[10.5px] bg-[rgba(57,255,159,0.06)] px-1 py-0.5 rounded">window.runtime.*</code>.</p>
          </div>
        </section>
      </div>
    </>
  );
}
