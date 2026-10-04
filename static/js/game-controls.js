(function () {
  const button = document.querySelector("[data-game-fire]");
  const frame = document.querySelector(".game-frame iframe");
  if (!button || !frame) return;

  let firing = false;

  function sendKey(type) {
    const gameWindow = frame.contentWindow;
    if (!gameWindow) return;
    const canvas = gameWindow.document.getElementById("canvas");
    if (!canvas) return;
    const GameKeyboardEvent = gameWindow.KeyboardEvent;
    canvas.dispatchEvent(new GameKeyboardEvent(type, {
      key: " ",
      code: "Space",
      keyCode: 32,
      which: 32,
      bubbles: true,
      cancelable: true
    }));
  }

  function startFiring(event) {
    event.preventDefault();
    if (firing) return;
    const canvas = frame.contentWindow && frame.contentWindow.document.getElementById("canvas");
    if (canvas) canvas.focus();
    firing = true;
    sendKey("keydown");
    if (event.pointerId !== undefined && button.setPointerCapture) {
      button.setPointerCapture(event.pointerId);
    }
  }

  function stopFiring() {
    if (!firing) return;
    firing = false;
    sendKey("keyup");
  }

  button.addEventListener("pointerdown", startFiring);
  button.addEventListener("pointerup", stopFiring);
  button.addEventListener("pointercancel", stopFiring);
  button.addEventListener("lostpointercapture", stopFiring);
  button.addEventListener("click", function (event) {
    if (event.detail !== 0 || firing) return;
    sendKey("keydown");
    window.setTimeout(function () { sendKey("keyup"); }, 100);
  });
  window.addEventListener("blur", stopFiring);
  document.addEventListener("visibilitychange", function () {
    if (document.hidden) stopFiring();
  });
})();
