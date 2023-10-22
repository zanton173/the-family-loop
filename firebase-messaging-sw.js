importScripts('https://www.gstatic.com/firebasejs/10.5.0/firebase-app.js');
importScripts('https://www.gstatic.com/firebasejs/10.5.0/firebase-messaging.js');
this.addEventListener("push", (event) => {

    const notification = event.data.json().notification

    event.waitUntil(this.registration.showNotification(notification.title, {
        body: notification.body,
        icon: "assets/apple-touch-icon.png"
    }));

});