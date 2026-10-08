<!-- pohw-badges:start -->
[![PoHW Editing Score](https://img.shields.io/badge/dynamic/json?url=https%3A%2F%2Fraw.githubusercontent.com%2Fmbb200291%2Fgo-occ-z%2Fpohw-badges%2Fsummary.json&query=%24.humanScore&suffix=%25&label=PoHW%20Editing%20Score&color=blue)](https://github.com/mbb200291/go-occ-z/actions/workflows/pohw-verify.yml)
[![PoHW Monitored](https://img.shields.io/badge/dynamic/json?url=https%3A%2F%2Fraw.githubusercontent.com%2Fmbb200291%2Fgo-occ-z%2Fpohw-badges%2Fsummary.json&query=%24.monitoredCoverage&suffix=%25&label=PoHW%20Monitored&color=informational)](https://github.com/mbb200291/go-occ-z/actions/workflows/pohw-verify.yml)
[![PoHW Receipt Verification](https://github.com/mbb200291/go-occ-z/actions/workflows/pohw-verify.yml/badge.svg?branch=dev)](https://github.com/mbb200291/go-occ-z/actions/workflows/pohw-verify.yml)
<!-- pohw-badges:end -->

# go-occ-z

This is an Go optimistic concurrency control implementation.
In backward validation manner.
Using skip-list (github.com/zhangyunhao116/skipmap) to ensure fast add, remove and ordered traverse.

Implement from the lecture: [Lecture #18: Timestamp Ordering Concurrency Control](https://15445.courses.cs.cmu.edu/fall2024/schedule.html)


> [!WARNING]
This project is currently under development and should not be used in production.
