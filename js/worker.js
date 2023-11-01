
onmessage = async (event) => {

    const result = event.data

    if (result[0] !== result[1]) {
        /*new Notification("Someone just made a new post!", {
            body: "There's a new post!",
            image: "../assets/android-chrome-512x512.png",
            icon: "../assets/favicon-32x32.png",
            tag: "newPostTag" + result[0]
        });*/
        const postBody = {
            "tag": result[0].toString()
        }
        const resp = await fetch("/send-new-posts-push", {
            method: "POST",
            headers: {
                "Content-Type": "application/json"
            },
            body: JSON.stringify(postBody)
        })

    }
}

this.addEventListener("push", (event) => {
    const notification = event.data.json();

    // {"title":"Hi" , "body":"something amazing!" , "url":"./?message=123"}
    console.log(notification)
    event.waitUntil(this.registration.showNotification(notification.title, {
        body: notification.body,
        tag: notification.tag,
        image: notification.image,
        icon: notification.icon,

    }));

});
/*
this.onpush = (event) => {
    console.log(event.data);
    const notification = event.data.json();
    // From here we can write the data to IndexedDB, send it to any open
    // windows, display a notification, etc.
   
};*/

async function getNewPosts() {
    const resp = await fetch("/new-posts", {
        method: "GET"
    })

}

