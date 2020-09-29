/** Global definitions for developement **/

// for style loader
declare module '*.css' {
  const styles: any;
  export = styles;
}

declare module "*.svg" {
  const content: any;
  export default content;
}