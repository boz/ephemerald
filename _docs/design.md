# Instance Lifecycle Hooks

1. live

  * check if container has started

1. init

  * create tables
  * insert data

1. reset

  * delete rows

TODO:

1. checkstart
1. init
1. checkready
1. reset

live -> init -> ready -> reset -> ready

# Instance Grouping

  * Pool 
    * _N_ running images for a distinct container configuration.

  * PoolSet
    * _N_ pools
    * each instance group has _N_ replicas

# use-cases

## "pod"/coupled images

rails - need pg, redis for every test

```yaml
- name: pg
  image: postgres
  size: 4

- name: redis
  image: redis
  size: 4
```

## anarchy

golang: checkout as needed.

```yaml
```

# API

## Pool

### Create pool

POST /pool -> types.Pool

### Delete pool

DELETE /pool/{pool-id}

### Checkout instance

POST /pool/{pool-id}/checkout -> types.Checkout

### Release instance

DELETE /pool/{pool-id}/checkout/{instance-id}


## Action-Complete

Local hook actions

```
PUT /pool/{pool-id}/action-complete
```

