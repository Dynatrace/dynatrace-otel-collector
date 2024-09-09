window.BENCHMARK_DATA = {
  "lastUpdate": 1725884408208,
  "repoUrl": "https://github.com/Dynatrace/dynatrace-otel-collector",
  "entries": {
    "Benchmark": [
      {
        "commit": {
          "author": {
            "name": "Dynatrace",
            "username": "Dynatrace"
          },
          "committer": {
            "name": "Dynatrace",
            "username": "Dynatrace"
          },
          "id": "8bc4789b009076ea7741c2c8ab31be4b9248c719",
          "message": "[chore]: add load test workflow",
          "timestamp": "2024-09-05T11:55:18Z",
          "url": "https://github.com/Dynatrace/dynatrace-otel-collector/pull/283/commits/8bc4789b009076ea7741c2c8ab31be4b9248c719"
        },
        "date": 1725884407702,
        "tool": "customSmallerIsBetter",
        "benches": [
          {
            "name": "cpu_percentage_avg",
            "value": 12.065510183493743,
            "unit": "%",
            "extra": "Log10kDPS/OTLP - Cpu Percentage"
          },
          {
            "name": "cpu_percentage_max",
            "value": 12.997850879011644,
            "unit": "%",
            "extra": "Log10kDPS/OTLP - Cpu Percentage"
          },
          {
            "name": "ram_mib_avg",
            "value": 67,
            "unit": "MiB",
            "extra": "Log10kDPS/OTLP - RAM (MiB)"
          },
          {
            "name": "ram_mib_max",
            "value": 94,
            "unit": "MiB",
            "extra": "Log10kDPS/OTLP - RAM (MiB)"
          },
          {
            "name": "dropped_span_count",
            "value": 0,
            "unit": "spans",
            "extra": "Log10kDPS/OTLP - Dropped Span Count"
          }
        ]
      }
    ]
  }
}