// The compass needles follow the cursor — the same easter egg as the app.
// The sprite holds 29 frames of the pixel compass, each with the red needle
// at a known bearing (degrees clockwise from north, measured per frame).
(function () {
  var ANGLES = [
    164.1, 191.1, 202.2, 229.3, 237.6, 248.7, 252.5, 269.0, 269.0, 269.0,
    285.2, 292.3, 300.2, 308.7, 336.6, 348.1, 17.1, 39.6, 43.3, 61.8, 71.6,
    73.5, 78.6, 90.9, 103.0, 110.6, 119.2, 138.4, 142.5,
  ];
  var SOURCE = 16; // native pixel-art frame size
  var NORTH = 15; // resting frame, closest to 0°

  var compasses = Array.prototype.map.call(
    document.querySelectorAll('[data-compass]'),
    function (el) {
      var size = parseInt(el.dataset.compass, 10) || 32;
      el.style.width = size + 'px';
      el.style.height = size + 'px';
      el.style.backgroundImage = "url('compass-sprite.png')";
      el.style.backgroundSize = ANGLES.length * size + 'px ' + size + 'px';
      el.style.imageRendering = 'pixelated';
      return { el: el, size: size, frame: -1 };
    }
  );
  if (!compasses.length) return;

  function setFrame(c, frame) {
    if (frame === c.frame) return;
    c.frame = frame;
    c.el.style.backgroundPosition = -frame * c.size + 'px 0';
  }
  compasses.forEach(function (c) {
    setFrame(c, NORTH);
  });

  var pending = null;
  document.addEventListener('mousemove', function (e) {
    if (pending) return;
    pending = requestAnimationFrame(function () {
      pending = null;
      compasses.forEach(function (c) {
        var r = c.el.getBoundingClientRect();
        var dx = e.clientX - (r.left + r.width / 2);
        var dy = e.clientY - (r.top + r.height / 2);
        if (dx * dx + dy * dy < 64) return; // dead zone over the icon
        var deg = ((Math.atan2(dx, -dy) * 180) / Math.PI + 360) % 360;
        var best = NORTH;
        var bestDist = Infinity;
        for (var i = 0; i < ANGLES.length; i++) {
          var d = Math.abs(((ANGLES[i] - deg + 540) % 360) - 180);
          if (d < bestDist) {
            bestDist = d;
            best = i;
          }
        }
        setFrame(c, best);
      });
    });
  });
})();
