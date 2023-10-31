
var receiveMoneroWallet = "49aUMmpPKq6dqe7ob1uVwsPdBghTNYZxST747RfjCoGS9xWC9MQFDQ2LrwqurmqVEsg9xagLksM4fKPpcreosRQ39bJ1CVA"
var notificationtype = ""
this.addEventListener("push", (event) => {

    const notification = event.data.json().notification
    //console.log(notification)
    event.waitUntil(this.registration.showNotification(notification.title, {
        actions: notification.actions,
        body: notification.body,
        icon: "assets/apple-touch-icon.png",
    }));
    notificationtype = notification.data.type

});
this.addEventListener("notificationclick", (event) => {
    event.notification.close()
    if (notificationtype == "event")
        clients.openWindow("/calendar")
    else
        clients.openWindow("/groupchat")
})