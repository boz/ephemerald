# Instance Lifecycle Hooks

1. initialize

  * create tables
  * insert data

1. healthcheck

  * connect to service

1. reset

  * delete rows

# Instance Grouping

  * Pool 
    * _N_ running images for a distinct container configuration.

  > * GroupPool
  >   * _N_ unique pools
  >   * network connectivity
  >   * aggregate state of children

  * PoolSet
    * _N_ pools
    * each instance group has _N_ replicas

# use-cases

## "pod"/coupled images

rails - need pg, redis for every test

```yaml
```

## anarchy

golang: checkout as needed.

```yaml
```

# API

## Create a pool set

```
POST /pool-set(/pool-name?)

-> pool-set-id
```

## Delete a pool set

```
DELETE /pool-set/{pool-set-id}

-> pool-set-id
```

## Checkout one instance of each item in a pool set

```
POST /pool-set/{pool-set-id}/checkout(/pool-name?)

-> checkout-id
```

## Release

```
DELETE /pool-set/{pool-set-id}/checkout/{checkout-id}

-> 
```

## Events

```
SUBSCRIBE /pool-set/{pool-set-id}
```

## Action-Complete

Local hook actions

```
PUT /pool/{pool-id}/action-complete
```

