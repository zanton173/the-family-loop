
onmessage = (event) => {

    const result = event.data
    postMessage(result)
}
self.addEventListener("push", (event) => {
    const notification = event.data.json();
    // {"title":"Hi" , "body":"something amazing!" , "url":"./?message=123"}
    console.log(event)
    event.waitUntil(self.registration.showNotification(notification.title, {
        body: notification.body,

    }));
});

self.addEventListener("notificationclick", (event) => {
    event.waitUntil(clients.openWindow(event.notification.data.notifURL));
});
/*navigator.serviceWorker.getRegistration().then((data) => data.pushManager.subscribe({
    userVisibleOnly: true
}).then(resp => console.log(resp)))*/

//navigator.serviceWorker.getRegistration("localhost").then