
onmessage = (event) => {

    const result = event.data
    postMessage(result)
}
this.addEventListener("push", (event) => {
    const notification = event.data.json();

    // {"title":"Hi" , "body":"something amazing!" , "url":"./?message=123"}
    console.log(notification)
    const notify = new Notification("testtiele")
    event.waitUntil(this.registration.showNotification(notification.title, {
        body: notification.body,

    }));
    notify.onshow = (event) => {
        console.log(event)
    }
});
/*
navigator.serviceWorker.getRegistration().then((data) => data.pushManager.subscribe({
    userVisibleOnly: true
}).then(resp => console.log(resp)))*/

