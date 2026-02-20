// TailChat GUI — Frontend Logic

// ---- State ----
let currentView = "peers";
let activeChatKey = "";
let peers = [];
let groups = [];
let pendingInvites = [];
let selfHostname = "";

// ---- Window Controls (frameless) ----
if (typeof runtime !== "undefined") {
  document.querySelectorAll('.title-bar-controls button').forEach(function (btn) {
    var label = btn.getAttribute("aria-label");
    if (label === "Minimize") btn.addEventListener("click", function () { runtime.WindowMinimise(); });
    if (label === "Maximize") btn.addEventListener("click", function () { runtime.WindowToggleMaximise(); });
    if (label === "Close") btn.addEventListener("click", function () { runtime.Quit(); });
  });
}

// ---- Theme ----
const themes = {
  "98": { css: "themes/98.css", bg: "theme-98" },
  "xp": { css: "themes/xp.css", bg: "theme-xp" },
  "7":  { css: "themes/7.css",  bg: "theme-7" },
};

function applyTheme(id) {
  const theme = themes[id];
  if (!theme) return;
  document.getElementById("theme-css").href = theme.css;
  document.body.className = theme.bg;
  document.getElementById("theme-select").value = id;
  localStorage.setItem("tailchat-theme", id);
}

document.getElementById("theme-select").addEventListener("change", function () {
  applyTheme(this.value);
});

// Load saved theme
applyTheme(localStorage.getItem("tailchat-theme") || "98");

// ---- View Switching ----
function showView(name) {
  document.querySelectorAll(".view").forEach(function (v) {
    v.style.display = "none";
  });
  var el = document.getElementById("view-" + name);
  if (el) {
    el.style.display = "";
  }
  currentView = name;
}

// ---- Wails Event Listeners ----
if (typeof runtime !== "undefined") {
  runtime.EventsOn("app:ready", function (info) {
    selfHostname = info.hostname;
    document.getElementById("self-hostname").textContent = info.hostname;
    document.getElementById("self-ip").textContent = info.ip;
    document.getElementById("title-text").textContent = "TailChat - " + info.hostname;
  });

  runtime.EventsOn("peers:updated", function (newPeers) {
    peers = newPeers || [];
    if (currentView === "peers") renderPeerList();
    updateStatusBar();
  });

  runtime.EventsOn("chat:message", function (data) {
    if (currentView === "chat" && data.chatKey === activeChatKey) {
      appendMessage(data);
    }
  });

  runtime.EventsOn("chat:groupInvite", function (data) {
    pendingInvites.push(data);
    if (currentView === "peers") renderInvites();
  });

  runtime.EventsOn("error", function (msg) {
    showErrorDialog(msg);
  });
}

// ---- Peer List ----
function renderPeerList() {
  var list = document.getElementById("peer-list");
  var onlineCount = peers.filter(function (p) { return p.online; }).length;
  document.getElementById("online-count").textContent = onlineCount;
  list.innerHTML = "";

  peers.forEach(function (peer) {
    if (peer.isSelf) return;
    var li = document.createElement("li");
    li.className = "peer-item";
    if (!peer.online) {
      li.className += " peer-offline";
    } else if (peer.runningTailchat) {
      li.className += " peer-tailchat";
    } else {
      li.className += " peer-online";
    }

    var nameSpan = document.createElement("span");
    nameSpan.textContent = peer.hostname;
    li.appendChild(nameSpan);

    var ipSpan = document.createElement("span");
    ipSpan.className = "peer-ip";
    ipSpan.textContent = peer.tailscaleIP;
    if (peer.online && peer.runningTailchat) {
      ipSpan.textContent += " \u2014 TailChat";
    }
    li.appendChild(ipSpan);

    li.addEventListener("dblclick", function () {
      connectOrOpen(peer);
    });

    list.appendChild(li);
  });

  renderGroups();
  renderInvites();
}

async function connectOrOpen(peer) {
  if (!peer.online) {
    showErrorDialog(peer.hostname + " is offline");
    return;
  }
  if (!peer.runningTailchat) {
    showErrorDialog(peer.hostname + " is not running TailChat");
    return;
  }

  try {
    var connected = await window.go.main.App.IsConnected(peer.hostname);
    if (connected) {
      openChat(peer.hostname);
      return;
    }
  } catch (e) {
    // Fall through to connect
  }

  showConnecting(peer.hostname);
  try {
    await window.go.main.App.ConnectToPeer(peer.tailscaleIP);
    openChat(peer.hostname);
  } catch (e) {
    showView("peers");
    showErrorDialog("Connection failed: " + e);
  }
}

function showConnecting(hostname) {
  document.getElementById("connecting-text").textContent =
    "Connecting to " + hostname + "...";
  document.querySelectorAll(".view").forEach(function (v) {
    v.style.display = "none";
  });
  document.getElementById("connecting-overlay").style.display = "";
  currentView = "connecting";
}

// ---- Chat ----
async function openChat(chatKey) {
  activeChatKey = chatKey;
  document.getElementById("chat-title").textContent = chatKey;
  document.getElementById("title-text").textContent =
    "TailChat - " + chatKey;

  var panel = document.getElementById("message-panel");
  panel.innerHTML = "";

  try {
    var messages = await window.go.main.App.GetMessages(chatKey);
    if (messages) {
      messages.forEach(function (msg) {
        appendMessage({
          sender: msg.sender,
          content: msg.content,
          timestamp: msg.timestamp,
          isOwn: msg.isOwn,
        });
      });
    }
  } catch (e) {
    // No history, that's fine
  }

  showView("chat");
  document.getElementById("message-input").focus();
  scrollToBottom();
}

function appendMessage(msg) {
  var panel = document.getElementById("message-panel");
  var div = document.createElement("div");
  div.className = "msg " + (msg.isOwn ? "msg-own" : "msg-peer");

  var time = "";
  try {
    var d = new Date(msg.timestamp);
    time = d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  } catch (e) {
    time = "??:??";
  }

  var timeSpan = document.createElement("span");
  timeSpan.className = "msg-time";
  timeSpan.textContent = "[" + time + "] ";

  var senderSpan = document.createElement("span");
  senderSpan.className = "msg-sender";
  senderSpan.textContent = msg.sender + ": ";

  var contentSpan = document.createElement("span");
  contentSpan.className = "msg-content";
  contentSpan.textContent = msg.content;

  div.appendChild(timeSpan);
  div.appendChild(senderSpan);
  div.appendChild(contentSpan);
  panel.appendChild(div);

  scrollToBottom();
}

function scrollToBottom() {
  var panel = document.getElementById("message-panel");
  panel.scrollTop = panel.scrollHeight;
}

async function sendMessage() {
  var input = document.getElementById("message-input");
  var content = input.value.trim();
  if (!content) return;
  input.value = "";

  try {
    if (activeChatKey.startsWith("group:")) {
      var groupID = activeChatKey.substring(6);
      await window.go.main.App.SendGroupMessage(groupID, content);
    } else {
      await window.go.main.App.SendMessage(activeChatKey, content);
    }
  } catch (e) {
    showErrorDialog("Send failed: " + e);
  }

  input.focus();
}

// ---- Groups ----
function renderGroups() {
  var fieldset = document.getElementById("groups-fieldset");
  var list = document.getElementById("group-list");

  if (groups.length === 0) {
    fieldset.style.display = "none";
    return;
  }

  fieldset.style.display = "";
  list.innerHTML = "";

  groups.forEach(function (g) {
    var li = document.createElement("li");
    li.className = "group-item";
    li.textContent = g.name + " (" + g.members.length + " members)";
    li.addEventListener("dblclick", function () {
      openChat("group:" + g.id);
    });
    list.appendChild(li);
  });
}

function renderInvites() {
  var fieldset = document.getElementById("invites-fieldset");
  var list = document.getElementById("invite-list");

  if (pendingInvites.length === 0) {
    fieldset.style.display = "none";
    return;
  }

  fieldset.style.display = "";
  list.innerHTML = "";

  pendingInvites.forEach(function (inv, idx) {
    var li = document.createElement("li");
    li.className = "invite-item";

    var text = document.createElement("span");
    text.textContent = inv.groupName + " from " + inv.from;
    li.appendChild(text);

    var acceptBtn = document.createElement("button");
    acceptBtn.textContent = "Accept";
    acceptBtn.addEventListener("click", function () {
      window.go.main.App.AcceptGroupInvite(
        inv.groupID,
        inv.groupName,
        inv.members,
        inv.from
      );
      groups.push({
        id: inv.groupID,
        name: inv.groupName,
        members: inv.members,
      });
      pendingInvites.splice(idx, 1);
      renderInvites();
      renderGroups();
    });
    li.appendChild(acceptBtn);

    list.appendChild(li);
  });
}

var groupMembers = [];

function showGroupCreate() {
  groupMembers = [];
  document.getElementById("group-name").value = "";
  document.getElementById("group-member-input").value = "";
  renderGroupMembers();
  showView("group-create");
}

function renderGroupMembers() {
  var list = document.getElementById("group-members-list");
  list.innerHTML = "";
  groupMembers.forEach(function (m, idx) {
    var li = document.createElement("li");
    li.className = "member-tag";
    li.textContent = m + " ";

    var removeBtn = document.createElement("button");
    removeBtn.textContent = "x";
    removeBtn.addEventListener("click", function () {
      groupMembers.splice(idx, 1);
      renderGroupMembers();
    });
    li.appendChild(removeBtn);
    list.appendChild(li);
  });
}

function addGroupMember() {
  var input = document.getElementById("group-member-input");
  var hostname = input.value.trim();
  if (hostname && groupMembers.indexOf(hostname) === -1) {
    groupMembers.push(hostname);
    renderGroupMembers();
  }
  input.value = "";
  input.focus();
}

async function createGroup() {
  var name = document.getElementById("group-name").value.trim();
  if (!name) {
    showErrorDialog("Group name is required");
    return;
  }
  if (groupMembers.length === 0) {
    showErrorDialog("Add at least one member");
    return;
  }

  try {
    await window.go.main.App.CreateGroup(name, groupMembers);
    // Refresh groups
    var g = await window.go.main.App.GetGroups();
    if (g) groups = g;
    showView("peers");
    renderGroups();
  } catch (e) {
    showErrorDialog("Failed to create group: " + e);
  }
}

// ---- Error Dialog ----
function showErrorDialog(msg) {
  document.getElementById("error-text").textContent = msg;
  document.getElementById("error-dialog").style.display = "";
}

function hideErrorDialog() {
  document.getElementById("error-dialog").style.display = "none";
}

// ---- Status Bar ----
function updateStatusBar() {
  var chatReady = peers.filter(function (p) { return p.online && !p.isSelf && p.runningTailchat; }).length;
  var online = peers.filter(function (p) { return p.online && !p.isSelf; }).length;
  document.getElementById("status-peers").textContent = chatReady + "/" + online + " peers";
}

// ---- Refresh ----
async function refreshPeers() {
  try {
    var p = await window.go.main.App.GetPeers();
    if (p) {
      peers = p;
      renderPeerList();
      updateStatusBar();
    }
  } catch (e) {
    showErrorDialog("Refresh failed: " + e);
  }
}

// ---- Event Handlers ----
document.getElementById("btn-send").addEventListener("click", sendMessage);
document.getElementById("message-input").addEventListener("keydown", function (e) {
  if (e.key === "Enter") {
    e.preventDefault();
    sendMessage();
  }
});

document.getElementById("btn-back").addEventListener("click", function () {
  document.getElementById("title-text").textContent = "TailChat - " + selfHostname;
  showView("peers");
  renderPeerList();
});

document.getElementById("btn-refresh").addEventListener("click", refreshPeers);
document.getElementById("btn-new-group").addEventListener("click", showGroupCreate);

document.getElementById("btn-add-member").addEventListener("click", addGroupMember);
document.getElementById("group-member-input").addEventListener("keydown", function (e) {
  if (e.key === "Enter") {
    e.preventDefault();
    addGroupMember();
  }
});

document.getElementById("btn-create-group").addEventListener("click", createGroup);
document.getElementById("btn-cancel-group").addEventListener("click", function () {
  showView("peers");
});

document.getElementById("btn-cancel-connect").addEventListener("click", function () {
  showView("peers");
});

document.getElementById("error-ok").addEventListener("click", hideErrorDialog);
document.getElementById("error-close-x").addEventListener("click", hideErrorDialog);

// Keyboard shortcut: Escape to go back
document.addEventListener("keydown", function (e) {
  if (e.key === "Escape") {
    if (document.getElementById("error-dialog").style.display !== "none") {
      hideErrorDialog();
    } else if (currentView === "chat") {
      document.getElementById("btn-back").click();
    } else if (currentView === "group-create") {
      showView("peers");
    }
  }
});
