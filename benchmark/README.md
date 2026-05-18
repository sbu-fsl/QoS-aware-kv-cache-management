# Benchmark result

Environment setup:

```yaml
# config.yaml
inference_engine:
  block_size: 16 # tokens per block
  compute_speed: 50.0 # blocks per second (simulated compute for a missing block)

cache_engine:
  tiers:
    - name: VRAM
      restore_speed: 100.0 # blocks per second
      capacity: 100 # max blocks

    - name: DRAM
      restore_speed: 20.0 # blocks per second
      capacity: 3000 # max blocks

    - name: Disk
      restore_speed: 5.0 # blocks per second
      capacity: 100000 # max blocks

# cmd.yaml
operations:
  - op: store
    blocks: "1-100000"

  - op: restore
    blocks: "90000-100000"

  - op: restore
    blocks: "5000-100000"

  - op: restore
    blocks: "1-100000"
```

Results:

| Policy          | OP  | Recompute (#) | VRAM (#) | DRAM (#) | Disk (#) | Latency (ms) |
| --------------- | :-: | :-----------: | :------: | :------: | :------: | :----------: |
| Default         |  2  |       0       |   100    |     1    |    0     |    1000      |
| QoS-no-limit    |  2  |      29       |    61    |    11    |    0     |     610      |
| QoS-200-cap     |  2  |      29       |    61    |    11    |    0     |     610      |
| QoS-1000-cap    |  2  |      29       |    61    |    11    |    0     |     610      |
| Default         |  3  |       0       |   100    |   901    |    0     |   45050      |
| QoS-no-limit    |  3  |     643       |   100    |   258    |    0     |   12900      |
| QoS-200-cap     |  3  |     200       |   100    |   701    |    0     |   35050      |
| QoS-1000-cap    |  3  |     643       |   100    |   258    |    0     |   12900      |
| Default         |  4  |       0       |   100    |   2900   |   7001   |   1400200    |
| QoS-no-limit    |  4  |     6600      |   100    |   2900   |   401    |    145000    |
| QoS-200-cap     |  4  |      200      |   100    |   2900   |   6801   |   1360200    |
| QoS-1000-cap    |  4  |      1000     |   100    |   2900   |   6001   |   1200200    |
| Default         |  5  |       0       |   100    |   2900   |  92001   |   18400200   |
| QoS-no-limit    |  5  |     83546     |   100    |   2900   |   8455   |   1691000    |
| QoS-200-cap     |  5  |      200      |   100    |   2900   |  91801   |   18360200   |
| QoS-1000-cap    |  5  |      1000     |   100    |   2900   |  91001   |   18200200   |
| Default         |  6  |       0       |   100    |   2900   |  97000   |   19400000   |
| QoS-no-limit    |  6  |     88090     |   100    |   2900   |   8910   |   1782000    |
| QoS-200-cap     |  6  |      200      |   100    |   2900   |  96800   |   19360000   |
| QoS-1000-cap    |  6  |      1000     |   100    |   2900   |  96000   |   19200000   |

## Improvements

Improvement is measured as latency reduction versus the `Default` policy for the same operation. Token distribution shows the share of restored tokens served by recomputation or each tier.

| Policy         | OP  | Latency Improvement | Recompute (%) | VRAM (%) | DRAM (%) | Disk (%) |
| -------------- | :-: | :-----------------: | :-----------: | :------: | :------: | :------: |
| Default        |  2  |       0.00%         |    0.00%      | 99.01%   |  0.99%   |  0.00%   |
| QoS-no-limit   |  2  |      39.00%         |   28.71%      | 60.40%   | 10.89%   |  0.00%   |
| QoS-200-cap    |  2  |      39.00%         |   28.71%      | 60.40%   | 10.89%   |  0.00%   |
| QoS-1000-cap   |  2  |      39.00%         |   28.71%      | 60.40%   | 10.89%   |  0.00%   |
| Default        |  3  |       0.00%         |    0.00%      |  9.99%   | 90.01%   |  0.00%   |
| QoS-no-limit   |  3  |      71.34%         |   64.24%      |  9.99%   | 25.77%   |  0.00%   |
| QoS-200-cap    |  3  |      22.18%         |   19.98%      |  9.99%   | 70.03%   |  0.00%   |
| QoS-1000-cap   |  3  |      71.34%         |   64.24%      |  9.99%   | 25.77%   |  0.00%   |
| Default        |  4  |       0.00%         |    0.00%      |  1.00%   | 29.00%   | 70.00%   |
| QoS-no-limit   |  4  |      89.64%         |   65.99%      |  1.00%   | 29.00%   |  4.01%   |
| QoS-200-cap    |  4  |       2.86%         |    2.00%      |  1.00%   | 29.00%   | 68.00%   |
| QoS-1000-cap   |  4  |      14.28%         |   10.00%      |  1.00%   | 29.00%   | 60.00%   |
| Default        |  5  |       0.00%         |    0.00%      |  0.11%   |  3.05%   | 96.84%   |
| QoS-no-limit   |  5  |      90.81%         |   87.94%      |  0.11%   |  3.05%   |  8.90%   |
| QoS-200-cap    |  5  |       0.22%         |    0.21%      |  0.11%   |  3.05%   | 96.63%   |
| QoS-1000-cap   |  5  |       1.09%         |    1.05%      |  0.11%   |  3.05%   | 95.79%   |
| Default        |  6  |       0.00%         |    0.00%      |  0.10%   |  2.90%   | 97.00%   |
| QoS-no-limit   |  6  |      90.82%         |   88.09%      |  0.10%   |  2.90%   |  8.91%   |
| QoS-200-cap    |  6  |       0.21%         |    0.20%      |  0.10%   |  2.90%   | 96.80%   |
| QoS-1000-cap   |  6  |       1.03%         |    1.00%      |  0.10%   |  2.90%   | 96.00%   |
