# Packages and plugins
In GoShimmer, new features are added through the [plugin system](plugin.md), however plugin objects should only expose the logic that is implemented in a different package.
The plugin system works as an adapter that can be used to easily expose complex logic through a common interface. 
It's really useful in a prototype software like GoShimmer, because it's possible to easily switch between different implementations just by using different plugin, without having to
rewrite the code using it. 
![Adapter design pattern](https://upload.wikimedia.org/wikipedia/commons/4/4e/Adapter_pattern.png "Adapter design pattern")

When creating new plugin, the logic should be implemented in a separate package stored in the `packages/` directory. 
The package should contain all struct and interface definitions used, as well as the specific logic. 
It should not reference any `plugin` packages from `plugin/` directory as this could lead to circular dependencies between packages.

There are no special interfaces or requirements that packages in the `packages/` directory are forced to follow. They should be independent of other packages if possible, 
to avoid problems due to changing interfaces in other packages.