var cm = new CodeMirror.fromTextArea(document.getElementById("score"), {lineNumbers: true});
const playerLink = document.createElement("a");
playerLink.href = "/player";
playerLink.className = "btn btn-create";
playerLink.textContent = "Play in browser";
playerLink.addEventListener("click", () => {
  try { sessionStorage.setItem("muse-player-score", cm.getValue()); } catch {}
});
document.getElementById("create").appendChild(playerLink);
