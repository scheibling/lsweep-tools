import requests, json, os
import requests_toolbelt as rt
from struct import pack

LOG_TO_FILE=True
LOGFILE='agent.log'
LS_SERVER_URL='http://localhost:8011/lsagent'

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

def log_request(target, headers, body):
    print_and_append('-------------------------------------------')
    print_and_append('POST ' + target + ' HTTP/2\r\n')
    print_and_append('\r\n'.join(ensure_str(x) + ': ' + ensure_str(y) for x, y in headers.items()) + '\r\n')
    print_and_append('\r\n' + ensure_str(str(body)      ))

def log_response(resp):
    print_and_append('RESPONSE RECEIVED:')
    print_and_append('HTTP/2 ' + str(resp.status_code) + ' ' + resp.reason + '\r\n')
    print_and_append('\r\n'.join(ensure_str(x) + ': ' + ensure_str(y) for x, y in resp.headers.items()) + '\r\n')
    print_and_append(ensure_str(resp.text))


def make_request(headers, datadict):
    mp_data = rt.MultipartEncoder(datadict)
    headers['Content-Type'] = mp_data.content_type
    
    log_request(
        target=LS_SERVER_URL,
        headers=headers,
        body=mp_data
    )
    
    req = requests.post(
        url=LS_SERVER_URL,
        headers=headers,    
        data=mp_data,
        verify=False
    )
    
    log_response(req)
    
    return req

def action_hello():
    make_request(
        headers={},
        datadict={
            'Action': 'Hello'
        }
    )
    
def action_assetstatus():
    make_request(
        headers={},
        datadict={
            'Action': 'AssetStatus',
            'OperatingSystem': 'Linux',
            'AssetId': '1234abcd-0883-472e-92ec-b4d3d1c59768'
        }
    )

def action_assetconfig():
    make_request(
        headers={},
        datadict={
            'Action': 'Config',
            'OperatingSystem': 'Linux',
            'AssetId': '1234abcd-0883-472e-92ec-b4d3d1c59768'
        }
    )

def action_sendscan():
    expath = '/'.join(
        os.path.realpath(__file__).split(os.path.sep)[:-2] + 
        ['examples/scan-result-min.json']
    )
    with open(expath, 'rb') as f:
        scandata = f.read()
    
    src_len = len(scandata)
    scandata += pack('<I', src_len)
    
    make_request(
        headers={},
        datadict={
            'Action': 'ScanData',
            'OperatingSystem': 'Linux',
            'AssetId': '1234abcd-0883-472e-92ec-b4d3d1c59768',
            'Scan': ('Scan', scandata, 'application/octet-stream')
        }
    )

if __name__ == '__main__':
    input("Press enter to send Hello request")
    action_hello()
    input("Press enter to send AssetStatus request")
    action_assetstatus()
    input("Press enter to send Config request")
    action_assetconfig()
    input("Press enter to send the scan data")
    action_sendscan()