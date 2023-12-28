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