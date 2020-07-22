/** Global definitions for developement **/

// for style loader
declare module "*.css" {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const styles: any;
  export = styles;
}

declare module "*.svg" {
  const content: string;
  export default content;
}