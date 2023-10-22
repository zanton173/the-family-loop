importScripts('https://www.gstatic.com/firebasejs/10.5.0/firebase-app.js');
importScripts('https://www.gstatic.com/firebasejs/10.5.0/firebase-messaging.js');
this.addEventListener("push", (event) => {

    const notification = event.data.json().notification
    console.log(event.data.json())
    event.waitUntil(this.registration.showNotification(notification.title, {
        body: notification.body,
        icon: "assets/apple-touch-icon.png"
    }));

});
this.addEventListener("message", ({ data }) => {
    if (data.action === "initializeFirebase") {
        if (firebase.messaging.isSupported()) {
            const { config } = data.payload;

            firebase.initializeApp(config);
            firebase.messaging();
        } else {
            alert("not supported")
        }
    }
});