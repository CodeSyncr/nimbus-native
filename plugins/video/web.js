// A <video> element in the Native view's box.
NN.view("video", (el, emit) => {
  const v = document.createElement("video");
  v.playsInline = true;
  v.style.cssText = "width:100%;height:100%;object-fit:contain;background:#000;display:block";
  v.addEventListener("loadedmetadata", () => emit("ready", { seconds: isFinite(v.duration) ? v.duration : 0 }));
  v.addEventListener("ended", () => emit("end", {}));
  el.appendChild(v);
  return (p) => {
    if (v.getAttribute("src") !== p.url) v.src = p.url;
    v.loop = !!p.loop;
    v.muted = !!p.muted;
    v.controls = !!p.controls;
    if (p.paused) v.pause();
    else v.play().catch(() => {}); // a browser may wait for a tap to play with sound
  };
});
