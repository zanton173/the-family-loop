
var notificationtype = ""
var notificationthread = ""
this.addEventListener("push", (event) => {

    const notification = event.data.json().notification
    console.log(notification)
    //console.log(notification)
    event.waitUntil(this.registration.showNotification(notification.title, {
        actions: notification.actions,
        body: notification.body,

        icon: notification.image,
        image: "/assets/icon-96x96.png",
    }));
    notificationtype = notification.data.type
    notificationthread = notification.data.thread

});
this.addEventListener("notificationclick", (event) => {

    event.notification.close()

    if (event.notification.data == "event")
        clients.openWindow("/calendar")
    else if (event.notification.data == "posts")
        clients.openWindow("/posts")
    else
        clients.openWindow("/groupchat?chatMessage=" + event.notification.body.replace(" ", "%20") + "&thread=" + notificationthread)
})