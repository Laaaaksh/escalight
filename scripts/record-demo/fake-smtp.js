// Minimal SMTP server so Escalight's real internal/notify/email.go client has
// a genuine relay to talk to during the recording, without needing real
// credentials or network mail delivery. Implements just enough of RFC 5321
// for Go's net/smtp.SendMail to complete a real send successfully.
"use strict";

const net = require("net");

function startFakeSmtp(host, port) {
  const messages = [];

  const server = net.createServer((socket) => {
    let buffer = "";
    let inData = false;
    let dataLines = [];

    socket.write("220 localhost escalight-demo-fake-smtp\r\n");

    socket.on("data", (chunk) => {
      buffer += chunk.toString("utf8");
      let idx;
      while ((idx = buffer.indexOf("\r\n")) !== -1) {
        const line = buffer.slice(0, idx);
        buffer = buffer.slice(idx + 2);

        if (inData) {
          if (line === ".") {
            inData = false;
            messages.push(dataLines.join("\n"));
            dataLines = [];
            socket.write("250 OK\r\n");
          } else {
            dataLines.push(line);
          }
          continue;
        }

        const cmd = line.split(" ")[0].toUpperCase();
        switch (cmd) {
          case "EHLO":
          case "HELO":
            socket.write("250 escalight-demo-fake-smtp\r\n");
            break;
          case "MAIL":
            socket.write("250 OK\r\n");
            break;
          case "RCPT":
            socket.write("250 OK\r\n");
            break;
          case "DATA":
            inData = true;
            socket.write("354 End data with <CR><LF>.<CR><LF>\r\n");
            break;
          case "QUIT":
            socket.write("221 Bye\r\n");
            socket.end();
            break;
          default:
            socket.write("250 OK\r\n");
        }
      }
    });
  });

  return new Promise((resolve, reject) => {
    server.on("error", reject);
    server.listen(port, host, () => resolve({ server, messages }));
  });
}

module.exports = { startFakeSmtp };
