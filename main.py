import logging
import os
import time
from datetime import datetime
from pathlib import Path

import requests
import secrets

ADMIN_KEY = secrets.ADMIN_KEY
CLIENT_KEY = secrets.CLIENT_KEY
BASE_URL = secrets.BASE_URL
BASE_PATH = os.path.abspath(os.path.dirname(__file__)) + '/downloads/'

CLIENT_HEADERS = {
    "Authorization": "Bearer " + CLIENT_KEY,
    "Accept": "application/json",
    "Content-Type": "application/json"
}

ADMIN_HEADERS = {
    "Authorization": "Bearer " + ADMIN_KEY,
    "Accept": "application/json",
    "Content-Type": "application/json"
}


def downloadfromurl(url, path, filename):
    Path(path).mkdir(parents=True, exist_ok=True)

    if not path.endswith("/"):
        path += "/"

    r = requests.get(url, stream=True)

    with open(path + filename, "wb") as file:
        for chunk in r.iter_content(chunk_size=52428800):
            if chunk:
                file.write(chunk)


def get_backups(serverid):
    url = BASE_URL + serverid + '/backups'

    response = requests.get(url, headers=CLIENT_HEADERS).json()

    return response


def delete_backup(serverid, backupuuid):
    url = BASE_URL + serverid + '/backups/' + backupuuid

    requests.delete(url, headers=CLIENT_HEADERS)


def create_backup(serverid):
    url = BASE_URL + serverid + '/backups'

    response = requests.post(url=url, headers=CLIENT_HEADERS).json()

    return response['attributes']['uuid']


def isbackupfinished(serverid, backupuuid):
    url = BASE_URL + serverid + '/backups/' + backupuuid

    response = requests.get(url, headers=CLIENT_HEADERS).json()

    return response['attributes']['completed_at']


def downlod_backup(backupuuid, serverid, servername):
    url = BASE_URL + serverid + '/backups/' + backupuuid + '/download'
    downloadurl = requests.get(url, headers=CLIENT_HEADERS).json()['attributes']['url']
    downloadpath = BASE_PATH + servername + '/'
    filename = servername + '-' + datetime.now().strftime('%Y-%m-%d_%H-%M-%S') + '.tar.gz'

    logging.info("Starting download of " + servername)
    downloadfromurl(downloadurl, downloadpath, filename)
    logging.info("Download completed")


def make_backups(servers):
    for i in servers:
        if i['feature_limits']['backups'] < 1:
            logging.info("Skipping backup for " + i['name'])
            continue

        if get_backups(i['identifier'])['meta']['pagination']['total'] >= i['feature_limits']['backups']:
            oldest_backup = get_backups(i['identifier'])['data'][0]['attributes']['uuid']
            delete_backup(i['identifier'], oldest_backup)

        logging.info("Creating backup of " + i['name'])
        backupuuid = create_backup(i['identifier'])

        while isbackupfinished(i['identifier'], backupuuid) is None:
            time.sleep(5)
        logging.info("Backup finished for " + i['name'])

        downlod_backup(backupuuid, i['identifier'], i['name'])


def get_servers():
    url = 'https://badsl.nl/api/application/servers'

    response = requests.get(url, headers=ADMIN_HEADERS).json()

    serverlistraw = response['data']
    servercount = response['meta']['pagination']['total']
    servers = []

    for i in range(servercount):
        servers.append(serverlistraw[i]['attributes'])

    return servers


def main():
    servers = get_servers()
    make_backups(servers)


main()
