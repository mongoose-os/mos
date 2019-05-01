# FWBuild role

## Deploying latest:

```
ansible-playbook -i hosts.yml fwbuild.yml -e build=yes
```

In this case, `fwbuild-instance` and `cloud-mos` will be deployed with the tag `latest`.

## Deploying release (in the example below, 1.5)

```
ansible-playbook -i hosts.yml fwbuild.yml -e build=yes -e mos_version_tag=1.5
```

In this case, `fwbuild-instance` and `cloud-mos` will be deployed with three tags: `1.5`, `release` and `latest`.

Note that setting `mos_version_tag` doesn't affect the version of `fwbuild-manager`, which is always `latest`.
