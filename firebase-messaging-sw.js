this.addEventListener("push", (event) => {

    const notification = event.data.json().notification
    event.waitUntil(this.registration.showNotification(notification.title, {
        actions: notification.actions,
        body: notification.body,
        icon: notification.image,
        image: "/assets/icon-512x512.png",
    }));

});
this.addEventListener("notificationclick", (event) => {
    if ((event.notification.data !== null && event.notification.data == "calendar") || (event.notification.actions[0] !== null && event.notification.actions[0].action == "calendar"))
        clients.openWindow("/calendar")
    else if ((event.notification.data !== null && event.notification.data == "posts") || (event.notification.actions[0] !== null && event.notification.actions[0].action == "posts"))
        clients.openWindow("/posts")
    else if ((event.notification.data !== null && event.notification.data == "pong") || (event.notification.actions[0] !== null && event.notification.actions[0].action == "pong"))
        clients.openWindow("/games/pong")
    else if ((event.notification.data !== null && event.notification.data == "groupchat") || (event.notification.actions[0] !== null && event.notification.actions[0].action == "groupchat"))
        clients.openWindow("/groupchat?thread=" + event.notification.actions[1].action)
    else
        console.log('Not sure where to go')
    event.notification.close()
})