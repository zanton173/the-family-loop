
var receiveMoneroWallet = "49aUMmpPKq6dqe7ob1uVwsPdBghTNYZxST747RfjCoGS9xWC9MQFDQ2LrwqurmqVEsg9xagLksM4fKPpcreosRQ39bJ1CVA"
this.addEventListener("push", (event) => {

    const notification = event.data.json().notification
    console.log(notification)

    event.waitUntil(this.registration.showNotification(notification.title, {
        actions: notification.actions,
        body: notification.body,
        icon: "assets/apple-touch-icon.png",
    }));

});
this.addEventListener("notificationclick", (event) => {
    event.notification.close()
    clients.openWindow("/groupchat")
})