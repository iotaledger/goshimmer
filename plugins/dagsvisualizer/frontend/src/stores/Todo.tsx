import {connectWebSocket} from 'WS';

export class Todo {
    constructor() {
        connectWebSocket("/ws",
        () => {console.log("connection opened")},
        () => {},
        () => {console.log("connection error")})
    } 
}

export default Todo;