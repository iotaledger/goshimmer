---
description: GoShimmer uses the adapter design pattern to easily switch between different implementations and internal interfaces just by using a different plugin, without having to rewrite the code using it.
image: /img/logo/goshimmer_light.png
keywords:
- dependency
- plugins
- plugin system
- code
- internal logic
- package
- adapter design pattern
- adapter
- circular dependency
---
# Dependency of Packages and Plugins

In GoShimmer, new features are added through the [plugin system](plugin.md).
When creating a new plugin, it must implement an interface shared with all other plugins, so it's easy to add new
plugins and change their internal implementation without worrying about compatibility. 
Because of this, to make the code clean and easily manageable the plugin's internal logic has to be implemented in a different package.
This is an example of an [adapter design pattern](https://en.wikipedia.org/wiki/Adapter_pattern) that is often used in plugin systems.
It's really useful in a prototype software like GoShimmer, because it's possible to easily switch between different implementations 
and internal interfaces just by using a different plugin, without having to rewrite the code using it. 

When creating a new plugin, the logic should be implemented in a separate package stored in the `packages/` directory. 
The package should contain all struct and interface definitions used, as well as the specific logic. 
It should not reference any `plugin` packages from the `plugin/` directory as this could lead to circular dependencies between packages.

There are no special interfaces or requirements that packages in the `packages/` directory are forced to follow. However, they should be independent of other packages if possible, 
to avoid problems due to changing interfaces in other packages.