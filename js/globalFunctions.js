function logoutFunction() {
    location.href = "/"
    document.cookie = "session_id=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;";
    fetch("/delete-jwt", {
        method: "POST",
        headers: {
            "Content-Type": "application/json"
        }
    })
    //window.location.reload()

}

async function getNotified() {
    !("Notification" in window)
        ? alert("This browser does not support desktop notification")
        : await Notification.requestPermission().then(() => {
            if (Notification.permission == 'granted') {

                getToken(messaging, { vapidKey: "BJmKY269Mkqw_zRnXy0n1ncFOBsamgi7hSpli4hKGlAJ-OKTae7qj8scasqrO9dpdmntNXXgbsMK3okY0bpOBVQ" })
                    .then((currentToken) => {

                        notificationBody = {
                            fcm_token: currentToken
                        }
                        fetch("/create-subscription", {
                            method: "POST",
                            headers: {
                                "Content-Type": "application/json"
                            },
                            body: JSON.stringify(notificationBody)
                        })
                            .then(() => {
                                return window.navigator.serviceWorker
                                    .getRegistration('/firebase-cloud-messaging-push-scope')
                                    .then((serviceWorker) => {
                                        if (serviceWorker) return serviceWorker;
                                        return window.navigator.serviceWorker.register('/firebase-messaging-sw.js', {
                                            scope: '/firebase-cloud-messaging-push-scope',
                                        });
                                    });
                            })
                    })
            }
        }).catch((err) => alert(err))

}