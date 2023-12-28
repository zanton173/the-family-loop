import { initializeApp } from "https://www.gstatic.com/firebasejs/10.5.2/firebase-app.js";

// TODO: Add SDKs for Firebase products that you want to use

// https://firebase.google.com/docs/web/setup#available-libraries

export const firebaseConfig = {
    apiKey: "AIzaSyBnHU_ureh5RmoYnzBkm7KZ2r-aIJcsOw8",
    authDomain: "the-family-loop-fb0d9.firebaseapp.com",
    projectId: "the-family-loop-fb0d9",
    storageBucket: "the-family-loop-fb0d9.appspot.com",
    messagingSenderId: "760310663988",
    appId: "1:760310663988:web:56e8290ddb1d11b2e77361"
}
const app = initializeApp(firebaseConfig)
export default app