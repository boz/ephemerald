# Instance Lifecycle Hooks

1. initialize

  * create tables
  * insert data

1. healthcheck

  * connect to service

1. reset

  * delete rows

# Instance Grouping

  * Instance
    * One running image

  * InstanceGroup
    * _N_ unique instances

  * InstanceSet
    * _N_ instance groups
    * each instance group has _N_ replicas
