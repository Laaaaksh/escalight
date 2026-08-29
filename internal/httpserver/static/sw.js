// Escalight service worker: shows incoming pages as push notifications with
// an "Acknowledge" action that fires directly from the notification, without
// opening the app first.

self.addEventListener("install", () => self.skipWaiting());
self.addEventListener("activate", (event) => event.waitUntil(self.clients.claim()));

self.addEventListener("push", (event) => {
  if (!event.data) return;
  let payload;
  try {
    payload = event.data.json();
  } catch {
    return;
  }

  event.waitUntil(
    self.registration.showNotification(payload.title || "Escalight", {
      body: payload.body || "",
      icon: "/static/icon-192.png",
      badge: "/static/icon-192.png",
      tag: payload.incidentId,
      requireInteraction: true,
      data: payload,
      actions: [
        { action: "acknowledge", title: "Acknowledge" },
        { action: "view", title: "View" },
      ],
    })
  );
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  const data = event.notification.data || {};

  if (event.action === "acknowledge" && data.ackUrl) {
    // Same-origin fetch from a service worker carries the browser's session
    // cookie, so this acknowledges the incident without opening a tab.
    event.waitUntil(fetch(data.ackUrl, { method: "POST", credentials: "same-origin" }));
    return;
  }

  const url = data.url || "/incidents";
  event.waitUntil(
    self.clients.matchAll({ type: "window" }).then((clients) => {
      for (const client of clients) {
        if (client.url.includes(url) && "focus" in client) return client.focus();
      }
      return self.clients.openWindow(url);
    })
  );
});
