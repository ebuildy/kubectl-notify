// Front-end for the kubectl-notify web UI: a single vertical timeline of event
// cards, newest at the top, ordered by event time. On load it fetches the
// buffered snapshot from /api/events, then streams new events over /ws. Each
// card always shows its time in the top-right corner.

(function () {
  "use strict";

  const board = document.getElementById("board");
  const statusEl = document.getElementById("status");

  function setStatus(text, cls) {
    statusEl.textContent = text;
    statusEl.className = "status status--" + cls;
  }

  function chip(text, cls) {
    const el = document.createElement("span");
    el.className = "chip" + (cls ? " " + cls : "");
    el.textContent = text;
    return el;
  }

  // eventDate resolves the event's time, falling back to "now" (arrival time)
  // so the timeline always has a timestamp to order by and display.
  function eventDate(ev) {
    if (ev.timestamp) {
      const d = new Date(ev.timestamp);
      if (!isNaN(d.getTime())) {
        return d;
      }
    }
    return new Date();
  }

  function renderCard(ev) {
    const when = eventDate(ev);

    const card = document.createElement("article");
    card.className =
      "card card--" + (ev.urgency === "critical" ? "critical" : "normal");
    // Used to keep the timeline ordered newest-first on insert.
    card.dataset.ts = String(when.getTime());

    const line = document.createElement("div");
    line.className = "card__line";

    const id = document.createElement("span");
    id.className = "card__id";
    id.textContent = (ev.kind || "") + "/" + (ev.name || "");
    line.appendChild(id);

    // Time is always shown, in the top-right corner of the card.
    const time = document.createElement("time");
    time.className = "card__time";
    time.dateTime = when.toISOString();
    time.textContent = when.toLocaleTimeString();
    time.title = when.toLocaleString();
    line.appendChild(time);
    card.appendChild(line);

    const reason = document.createElement("div");
    reason.className = "card__reason";
    reason.textContent = ev.reason || "";
    card.appendChild(reason);

    const msg = document.createElement("div");
    msg.className = "card__msg";
    msg.textContent = ev.message || "";
    card.appendChild(msg);

    const chips = document.createElement("div");
    chips.className = "chips";
    if (ev.namespace) {
      chips.appendChild(chip(ev.namespace, "chip--ns"));
    }
    if (ev.labels) {
      Object.keys(ev.labels).forEach(function (k) {
        chips.appendChild(chip(k + "=" + ev.labels[k]));
      });
    }
    if (chips.childNodes.length > 0) {
      card.appendChild(chips);
    }
    return card;
  }

  // renderItem wraps a card with a left timeline rail showing the event's hour
  // and day, plus a dot on the continuous timeline line.
  function renderItem(ev) {
    const when = eventDate(ev);

    const item = document.createElement("div");
    item.className = "timeline-item";
    item.dataset.ts = String(when.getTime());

    const rail = document.createElement("div");
    rail.className = "rail";

    const dot = document.createElement("span");
    dot.className =
      "rail__dot" + (ev.urgency === "critical" ? " rail__dot--critical" : "");
    rail.appendChild(dot);

    const hour = document.createElement("div");
    hour.className = "rail__time";
    hour.textContent = when.toLocaleTimeString([], {
      hour: "2-digit",
      minute: "2-digit",
    });
    rail.appendChild(hour);

    const day = document.createElement("div");
    day.className = "rail__day";
    day.textContent = when.toLocaleDateString([], {
      month: "short",
      day: "numeric",
    });
    rail.appendChild(day);

    item.appendChild(rail);
    item.appendChild(renderCard(ev));
    return item;
  }

  // addEvent inserts an item into the single vertical timeline, keeping it
  // ordered newest-first by event time.
  function addEvent(ev) {
    const item = renderItem(ev);
    const ts = Number(item.dataset.ts);

    let ref = board.firstChild;
    while (ref && Number(ref.dataset.ts) >= ts) {
      ref = ref.nextSibling;
    }
    board.insertBefore(item, ref);
  }

  function connectWS() {
    const proto = location.protocol === "https:" ? "wss:" : "ws:";
    const ws = new WebSocket(proto + "//" + location.host + "/ws");

    ws.onopen = function () {
      setStatus("live", "open");
    };
    ws.onmessage = function (msg) {
      try {
        addEvent(JSON.parse(msg.data));
      } catch (e) {
        /* ignore malformed frame */
      }
    };
    ws.onclose = function () {
      setStatus("disconnected — retrying", "closed");
      setTimeout(connectWS, 2000);
    };
    ws.onerror = function () {
      ws.close();
    };
  }

  // Initial load: render the buffered history, then start the live stream.
  fetch("/api/events")
    .then(function (r) {
      return r.json();
    })
    .then(function (events) {
      (events || []).forEach(addEvent);
    })
    .catch(function () {
      /* snapshot is best-effort; the WS stream still populates the board */
    })
    .finally(connectWS);
})();
