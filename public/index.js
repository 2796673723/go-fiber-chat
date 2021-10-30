window.onload = async () => {
  //初始化历史信息
  fetch(`http://${window.location.host}/api/info_list`)
    .then((res) => res.json())
    .then((res) => {
      let message = document.querySelector("#message");
      for (const item of res) {
        message.innerHTML += `${item}<br>`;
      }
    });
  //websocket服务
  let uri = `ws://${window.location.host}/api/ws`;
  let ws = new WebSocket(uri);
  ws.addEventListener("message", (e) => {
    document.querySelector("#message").innerHTML += `${e.data}<br>`;
    console.log(e.data);
  });
  let userName = document.querySelector("#name");
  let context = document.querySelector("#context");
  let send = document.querySelector("#send");
  send.onclick = () => {
    if (context.value !== "" && userName.value !== "") {
      let sendMessage = `${userName.value} : ${context.value}`;
      ws.send(sendMessage);
      context.value = "";
    }
  };
};
