import logging
import time
import requests

import secrets

CLIENT_URL = secrets.BASE_URL + "/api/client/servers/"
ADMIN_URL = secrets.BASE_URL + "/api/application/"

HEADERS = {
    "Authorization": "Bearer " + secrets.CLIENT_KEY,
    "Accept": "application/json",
    "Content-Type": "application/json"
}


def get_backups(serverid):
    url = CLIENT_URL + serverid + '/backups'

    response = requests.get(url, headers=HEADERS).json()

    return response


def delete_backup(serverid, backupuuid):
    url = CLIENT_URL + serverid + '/backups/' + backupuuid

    requests.delete(url, headers=HEADERS)


def create_backup(serverid):
    url = CLIENT_URL + serverid + '/backups'

    response = requests.post(url=url, headers=HEADERS).json()

    return response['attributes']['uuid']


def isbackupfinished(serverid, backupuuid):
    url = CLIENT_URL + serverid + '/backups/' + backupuuid

    response = requests.get(url, headers=HEADERS).json()

    return response['attributes']['completed_at']


def make_backups(servers):
    for i in servers:
        if i['feature_limits']['backups'] < 1:
            logging.info("Skipping backup for " + i['name'])
            continue

        if get_backups(i['identifier'])['meta']['pagination']['total'] >= i['feature_limits']['backups']:
            oldest_backup = get_backups(i['identifier'])[
                'data'][0]['attributes']['uuid']
            delete_backup(i['identifier'], oldest_backup)

        logging.info("Creating backup of " + i['name'])
        backupuuid = create_backup(i['identifier'])

        while isbackupfinished(i['identifier'], backupuuid) is None:
            time.sleep(5)
        logging.info("Backup finished for " + i['name'])


def get_servers():
    response = requests.get(ADMIN_URL + "servers",
                            headers=HEADERS).json()

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
