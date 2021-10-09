---
slug: new-release-v0.7.7
title: New Release v0.7.7
author: Luca Moser
author_title: Senior Software Engineer @ IOTA Foundation
author_url: https://github.com/luca-moser
author_image_url: https://images4.bamboohr.com/86711/photos/85-4-4.jpg?Policy=eyJTdGF0ZW1lbnQiOlt7IlJlc291cmNlIjoiaHR0cHM6Ly9pbWFnZXM0LmJhbWJvb2hyLmNvbS84NjcxMS8qIiwiQ29uZGl0aW9uIjp7IkRhdGVHcmVhdGVyVGhhbiI6eyJBV1M6RXBvY2hUaW1lIjoxNjMyMzEzNjUxfSwiRGF0ZUxlc3NUaGFuIjp7IkFXUzpFcG9jaFRpbWUiOjE2MzQ5MDU2NjF9fX1dfQ__&Signature=hFmmIsMq6ixckr-MdEq-GF5sZ1kZHGl0mgpZLd3IpYznswOq9xkiNHeQk56eqZgHteIXrvvF48MOJDw~t2-5W~4gUjdXz638SShuCyUfuqEwAz8Ms68h1dloNwL7wfcN4X4TVb75u-aBcZcVguQOAL-KBj-0UUT9lJrUjfm6njSVH~ir3KdPQmFrH52UXSRBnOvjpYfKJ-2ep-izZpkWvgEDB~nOQ-ztB5WtLRxaV4EgxT8HW5O4rwDlL0N7ZLrjs5OvSJjgwvYvhSwAVIrEaiqfUY8OPVnawzCRDZ1LYSPvWWWsBjJOlNbXy6JUsBRNzY0ncKFxKZkHTPtxty1I3g__&Key-Pair-Id=APKAIZ7QQNDH4DJY7K4Q
tags: [release]
---
# GoShimmer v0.7.7

:::note
This release does **not** include changes to the consensus mechanism and still uses FPC+FCoB.
:::

:::caution
This is a **breaking** maintenance release. You must delete your current database and upgrade your node to further participate in the network.
:::

The snapshot has been taken at 2021-10-08 2pm CEST.

Changelog:
- Changes the way the faucet plugin manages outputs in order to be able to service more funding requests.
- Fixes a nil pointer caused when a re-org is detected but the `RemoteLog` plugin is not enabled.
- Fixes an issue where the CLI wallet would no longer work under Windows.
- Use Go 1.17.2 docker image