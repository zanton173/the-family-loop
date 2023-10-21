this.addEventListener("push", (event) => {

    const notification = event.data.json().notification
    console.log(event.data.json())
    event.waitUntil(this.registration.showNotification(notification.title, {
        body: notification.body,
        icon: "assets/apple-touch-icon.png"
    }));

});