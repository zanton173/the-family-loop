onmessage = (event) => {

    const result = event.data

    if (result[0] !== result[1] && result[0] != null) {
        new Notification("Someone just made a new post!", {
            body: "There's a new post!",
            icon: "../assets/android-chrome-512x512.png",
            tag: "newPostTag" + result[0]
        });
    }
}

this.addEventListener("push", (event) => {
    const notification = event.data.json();

    // {"title":"Hi" , "body":"something amazing!" , "url":"./?message=123"}
    console.log(notification)

    event.waitUntil(this.registration.showNotification(notification.title, {
        body: notification.body,
    }));

});

async function getNewPosts() {
    const resp = await fetch("/new-posts", {
        method: "GET"
    })

}

