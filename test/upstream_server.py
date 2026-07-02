#!/usr/bin/env python3
"""Simple upstream HTTP server for WAF testing"""

from http.server import HTTPServer, BaseHTTPRequestHandler
import json

class TestHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.send_header('Content-Type', 'text/html')
        self.end_headers()
        
        html = f"""<!DOCTYPE html>
<html>
<head><title>Test Upstream</title></head>
<body>
    <h1>Welcome to Vinahost WAF Test Server</h1>
    <p>Path: {self.path}</p>
    <p>Headers:</p>
    <pre>{json.dumps(dict(self.headers), indent=2)}</pre>
</body>
</html>"""
        self.wfile.write(html.encode())
    
    def do_POST(self):
        content_length = int(self.headers.get('Content-Length', 0))
        body = self.rfile.read(content_length)
        
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        
        response = {
            'status': 'ok',
            'path': self.path,
            'body_received': body.decode('utf-8', errors='replace')
        }
        self.wfile.write(json.dumps(response, indent=2).encode())
    
    def log_message(self, format, *args):
        print(f"[UPSTREAM] {args[0]}")

if __name__ == '__main__':
    server = HTTPServer(('127.0.0.1', 8081), TestHandler)
    print("Upstream server listening on http://127.0.0.1:8081")
    server.serve_forever()
