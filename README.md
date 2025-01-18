# EVCC Tempo prices retrieval

Retrieve next days Tempo colors and transform the result into evcc compatible
tariffs.

## Usage

simplest way is to use the container and start it.

you can retrieve the prices in `/prices` endpoint.


## Configuration

You need to give a config file in order to make it work:

```yaml
# see below to fill these two values
clientid:
clientsecret:
prices:
  blue:
    peak: 0.1609
    off-peak: 0.1296
  white:
    peak: 0.1894
    off-peak: 0.1486
  red:
    peak: 0.7562
    off-peak: 0.1568
```

This configuration needs to be put in `/etc/evcc-tempo/evcc-tempo.yaml` or you
need to give the path in `CONFIG_FILE` environment variable.

### Create an application to connect to RTE API service

datas are directly retried from [RTE API](https://data.rte-france.com/) and you
need credentials in order to use it.

* [Create an account](https://data.rte-france.com/create_account).
* Search for `Tempo Like Supply Contract` in the API catalog (in
  [Consommation](https://data.rte-france.com/catalog/consumption) category) and
  click on `Découvrir l'API`.
* Once on Tempo API page, click on `Abonnez-vous à l'API` and create an
  application (or use an already created one).
* Once the application is created and the Tempo API associated, retrieve
  authentication information in your
  [application](https://data.rte-france.com/group/guest/apps): `ID Client` and
  `ID Secret`.
