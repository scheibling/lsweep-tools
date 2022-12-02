import io
import os
import cgi
import sys
import json
import gzip
from time import sleep
from struct import unpack
from requests_toolbelt import MultipartEncoder
from dataclasses import dataclass
from http.server import CGIHTTPRequestHandler, HTTPServer

LOG_TO_FILE=True
LOGFILE='server.log'
HOSTNAME = "example.com"
LISTEN = "0.0.0.0"
LISTENPORT = 8011
    
if len(sys.argv) > 1:
    HOSTNAME = sys.argv[1]
    if len(sys.argv) > 2:
        LISTENPORT = int(sys.argv[2])
    
def ensure_bytes(s):
    if isinstance(s, str):
        return s.encode('utf-8')
    return s

def ensure_str(s):
    if isinstance(s, bytes):
        return s.decode('utf-8')
    if isinstance(s, dict):
        return '\r\n'.join([ensure_str(x) + ': ' + ensure_str(y) for x, y in s.items()])
    if isinstance(s, (list, set, tuple)):
        return '[' + ', '.join(ensure_str(x) for x in s) + ']'
    return s

def recurse_ensure_str(d):
    if isinstance(d, dict):
        return {k: recurse_ensure_str(v) for k, v in d.items()}
    elif isinstance(d, (list, set, tuple)):
        return [recurse_ensure_str(v) for v in d]

    return ensure_str(d)

def print_and_append(s):
    print(s)
    if LOG_TO_FILE:
        with open(LOGFILE, 'a') as f:
            f.write(recurse_ensure_str(s) + '\n')

def log_request(req, pvars, raw_body):
    print_and_append('-------------------------------------------')
    print_and_append('POST /lsagent HTTP/2\r\n')    
    print_and_append('\r\n'.join(recurse_ensure_str(x) + ': ' + recurse_ensure_str(y) for x, y in req.headers.items()) + '\r\n')
    print_and_append(
        '\r\n' + 
        '\r\n'.join(
            [recurse_ensure_str(x) + ': ' + json.dumps(recurse_ensure_str(y)) for x, y in pvars.items()]
        ) + '\r\n')
    

def log_response(resp):
    print_and_append('RESPONSE SENT:')
    print_and_append('HTTP/2 ' + str(resp.status_code) + ' ' + resp.reason + '\r\n')
    print_and_append('\r\n'.join(ensure_str(x) + ': ' + ensure_str(y) for x, y in resp.headers.items()) + '\r\n')
    print_and_append(ensure_str(resp.body))
@dataclass
class Response:
    status_code: int
    reason: str
    headers: str
    body: str

class ResponseMaker:
    def __init__(self, action):
        self.action = getattr(self, 'get_' + action.lower())
        self.resp_headers = {
            'Server': 'Microsoft-HTTPAPI/2.0'
        }
        self.multipart_response = False
    
    def send_response(self, req, pvars, raw_body):
        resp = self.action(req, pvars, raw_body)
        
        req.send_response(resp.status_code, resp.reason)
        for k, v in resp.headers.items():
            req.send_header(k, v)

        req.end_headers()
        write_body = resp.body
        if isinstance(write_body, (str, bytes)):
            write_body = [write_body]
        req.wfile.writelines(write_body)
        
        log_response(resp)
        return
    
    def get_hello(self, req, pvars, raw_body):
        return Response(
            status_code=200,
            reason='OK',
            headers=self.resp_headers,
            body=[b'OK', b'\r\n', b'']
        )
    
    def get_assetstatus(self, req, pvars, raw_body): 
        multipart = MultipartEncoder(
            fields={
                "Status": "Enabled"
            }
        )
        
        return Response(
            status_code=200,
            reason='OK',
            headers={
                'Content-Type': multipart.content_type
            },
            body=multipart.to_string()
        )
    
    def get_config(self, req, pvars, raw_body):
        thepath = '/'.join(os.path.realpath(__file__).split(os.path.sep)[:-2] + ['examples/scan-configuration.xml'])
        with open(thepath, 'rb') as file:
            config = file.read()

        config = config.replace(b'[LANSWEEPER-URL-PLACEHOLDER]', ensure_bytes(HOSTNAME) + b':' + ensure_bytes(str(LISTENPORT)))

        multipart = MultipartEncoder(
            fields={
                'Config': ('Config', config, 'application/octet-stream')
            }
        )
        
        return Response(
            status_code=200,
            reason='OK',
            headers={
                'Content-Type': multipart.content_type
            },
            body=multipart.to_string()
        )
    
    def get_scandata(self):
        return Response(
            status_code=200,
            reason='OK',
            headers=self.resp_headers,
            body=[b'OK', b'\r\n', b'']
        )

class MocksweeperHandler(CGIHTTPRequestHandler):
    def _parse_body(self):
        content_length = int(self.headers.get('Content-Length', 0))
        raw_body = b''
        
        while len(raw_body) < content_length:
            sleep(1)
            raw_body += self.rfile.read(content_length)
        
        ctype, pdict = cgi.parse_header(self.headers.get('content-type', {}))
        if pdict.get('boundary', None) is None:
            pdict['boundary'] = io.BytesIO(raw_body).readline().rstrip(b'\r\n')[2:]
        
        pdict['boundary'] = ensure_bytes(pdict['boundary'])
        pvars = cgi.parse_multipart(io.BytesIO(raw_body), pdict)

        log_request(self, pvars, raw_body)
        
        return pvars, raw_body

    def do_POST(self):
        postvars, raw = self._parse_body()
        
        return ResponseMaker(postvars['Action'][0]).send_response(self, postvars, raw)
        
if __name__ == '__main__':
    server = HTTPServer((LISTEN, LISTENPORT), MocksweeperHandler)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    
    server.server_close()