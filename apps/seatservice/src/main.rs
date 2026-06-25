use std::env;
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};

fn main() -> std::io::Result<()> {
    let addr = env::var("VELOX_HTTP_ADDR").unwrap_or_else(|_| "0.0.0.0:8080".to_string());
    let listener = TcpListener::bind(&addr)?;
    println!("seatservice placeholder listening on {addr}");
    for stream in listener.incoming() {
        match stream {
            Ok(stream) => handle(stream)?,
            Err(err) => eprintln!("seatservice accept error: {err}"),
        }
    }
    Ok(())
}

fn handle(mut stream: TcpStream) -> std::io::Result<()> {
    let mut buffer = [0_u8; 1024];
    let read = stream.read(&mut buffer)?;
    let request = String::from_utf8_lossy(&buffer[..read]);
    let (status, body) = if request.starts_with("GET /healthz ") {
        ("200 OK", r#"{"status":"ok","service":"seatservice"}"#)
    } else {
        ("404 Not Found", r#"{"error":"not_found"}"#)
    };
    write!(
        stream,
        "HTTP/1.1 {status}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
        body.len()
    )
}
