
this.addEventListener("push", (event) => {

    const notification = event.data.json().notification

    event.waitUntil(this.registration.showNotification(notification.title, {
        body: notification.body,
        icon: "assets/apple-touch-icon.png"
    }));

});