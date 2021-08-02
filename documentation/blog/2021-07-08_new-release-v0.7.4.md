---
slug: new-release-v0.7.4
title: New Release v0.7.4
author: Dr. Angelo Capossele
author_title: Senior Research Scientist @ IOTA Foundation
author_url: https://github.com/capossele
author_image_url: https://images4.bamboohr.com/86711/photos/95-0-4.jpg?Policy=eyJTdGF0ZW1lbnQiOlt7IlJlc291cmNlIjoiaHR0cHM6Ly9pbWFnZXM0LmJhbWJvb2hyLmNvbS84NjcxMS8qIiwiQ29uZGl0aW9uIjp7IkRhdGVHcmVhdGVyVGhhbiI6eyJBV1M6RXBvY2hUaW1lIjoxNjI2MjE0MTI2fSwiRGF0ZUxlc3NUaGFuIjp7IkFXUzpFcG9jaFRpbWUiOjE2Mjg4MDYxMzZ9fX1dfQ__&Signature=J1tDKQcFzxJWikrPqGfkunFch~5KTUkr8g0LqYCiJrYnoqtSY1IbQAsgVrlKru7idSIABzOe3IB~lgNaaqJnBB9DJ8-yZnCxvvT1wdZkE8ov1bmgBJI1dJ5nc5WqJcOLyazP9JTG7zxDrbwj8VMYL1V-Q2HGEkx1RDuXXUJ3w8zqP9fOVscUkKh9YRG-b62LnZITOu2or0ZzbFOwEU-mRU6~7a4nH-FYkiSrePXv7SrsZ0ai2hEd-KGmk0~aOWMJ8vSesskUypEZIoGSekulO5t9vRISvW3zE2UlAFFJPsIQlBweEnXw3ss~FhaCXSbcypumu5279K2J5hWGm8kLHQ__&Key-Pair-Id=APKAIZ7QQNDH4DJY7K4Q
tags: [release]     
---
# GoShimmer v0.7.4

We have released GoShimmer v0.7.4:

- Add ParametersDefinition structs in integration test framework
- Add UTXO-DAG interface
- Fix setting correct properties to newly forked branches
- Fix using weak parents for direct approvers when looking for transactions that are approved by a message
- Update entry node from community
- Update docs
- Update snapshot file with DevNet UTXO at 2021-07-08 07:09 UTC

:::warning
Breaking changes: You must update your GoShimmer installation if you want to keep participating in the network. Follow the steps in the [official documentation](http://goshimmer.docs.iota.org/tutorials/setup.html#managing-the-goshimmer-node-lifecycle) to upgrade your node. 
:::

For those compiling from source, make sure to download the latest snapshot that we have updated. You can find more info on how to do that in the README.

We have also updated the domain name for the entry node from the community, make sure to update your config.