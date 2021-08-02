# Website

This website is built using [Docusaurus 2](https://docusaurus.io/), a modern static website generator.

## Installation

```console
yarn install
```

## Local Development

```console
yarn start
```

This command starts a local development server and opens up a browser window. Most changes are reflected live without having to restart the server.

## Build

```console
yarn build
```

This command generates static content into the `build` directory and can be served using any static contents hosting service.


# Adding content

## Docs

All the project documentation should be placed in the `docs` folder. If you wish to create a new document, you should create a new `.md` file in the corresponding folder.  For example, if you wanted to add a new tutorial, you should create a new `.md` file in `docs/tutorials`:

```
documentation # Root directory of your site
└── blog
└── docs
   ├── welcome.md
   └── tutorials
      └── your_new_tutorials_name.md

```

You can find more information on docs in the [official docusaurus documentation](https://docusaurus.io/docs/docs-introduction).

## Blog

You should store all your blog posts in the `blog` directory.  When creating a new blog post, please make sure to respect the file name structure which includes the posts date in YYYY_MM_DD format.  For example, if you wanted to add a post dated July 28th, 2021, your new `.md` file should be prefixed with `2021_07_28`. 

You can find more information on blog posts in the [official docusaurus documentation](https://docusaurus.io/docs/blog).

## Sidebar

As the project has multiple documentation pages and sections, these need to be organized.  This is handled by the `sidebars.js` file. The `sidebars.js` file contain an ordered JSON formatted object which will be used to generate the project sidebar.  

### Documents

You can add a new doc by adding a new object with type `doc` to the sidebar object:

```json
  {
    type: 'doc',
    label: 'FAQ',
    id: 'tutorials/your_new_tutorials_name',
  }
```
where

* `type` should always be 'doc'. 
* `label` should be your desired sidebar item's label.
* `id` is the item's identifier.  The ID field should contain the parent folder/s, if any.

### Categories

You can add a new category by adding a new object with type `category` to the sidebar object: 

```json
{
    type: 'category',
    label: 'Tutorials',
    items: []
}
```

where

* `type` should always be 'category'. 
* `label` should be your desired sidebar category's label.
* `items` is an array of [documents](#documents).

You can find more information on the sidebar and its components in the [official docusaurus documentation](https://docusaurus.io/docs/sidebar).
