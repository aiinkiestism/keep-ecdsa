import path from "path"
import pAll from "p-all"
import low from "lowdb"
import FileSync from "lowdb/adapters/FileSync.js"

import { getPastEvents, callWithRetry } from "./contract-helper.js"
import { KeepStatus, getKeepStatus } from "./keep.js"

const DATA_DIR_PATH = path.resolve(process.env.DATA_DIR_PATH || "./data")
const CACHE_PATH = path.resolve(DATA_DIR_PATH, "cache.json")

// Our expectation on how deep can chain reorganization be. We need this
// parameter because the previous cache refresh could store data that
// are no longer valid due to a chain reorganization. To overcome this
// problem we lookup `REORG_DEPTH_BLOCKS` earlier than the last refresh
// block when starting the cache refresh.
const REORG_DEPTH_BLOCKS = 12

const CONCURRENCY_LIMIT = 3

export default class Cache {
  constructor(web3, contracts) {
    this.web3 = web3
    this.contracts = contracts
  }

  async initialize() {
    this.cache = low(new FileSync(CACHE_PATH))
    await this.cache
        .defaults({
          keeps: [],
          lastRefreshBlock: this.contracts.factoryDeploymentBlock,
        })
        .write()
  }

  async refresh() {
    const factory = await this.contracts.BondedECDSAKeepFactory.deployed()

    const previousRefreshBlock = this.cache.get("lastRefreshBlock").value()

    const startBlock =
        previousRefreshBlock - REORG_DEPTH_BLOCKS > 0
            ? previousRefreshBlock - REORG_DEPTH_BLOCKS
            : 0

    console.log(
        `Looking for keeps created since block [${startBlock}]...`
    )

    const keepCreatedEvents = await getPastEvents(
        this.web3,
        factory,
        "BondedECDSAKeepCreated",
        startBlock
    )

    const newKeeps = []
    keepCreatedEvents.forEach((event) => {
      newKeeps.push({
        address: event.returnValues.keepAddress,
        members: event.returnValues.members,
        creationBlock: event.blockNumber,
      })
    })

    const cachedKeepsCount = this.cache.get("keeps").value().length

    console.log(
        `Number of keeps created since block ` +
        `[${startBlock}]: ${newKeeps.length}`
    )

    console.log(`Number of keeps in the cache: ${cachedKeepsCount}`)

    const actions = []
    newKeeps.forEach((keep) => {
      const isCached = this.cache
        .get("keeps")
        .find({ address: keep.address })
        .value()

      if (!isCached) {
        actions.push(() => this.fetchKeep(keep))
      }
    })

    if (actions.length === 0) {
      console.log("Cached keeps list is up to date")
    } else {
      console.log(
          `Fetching information about [${actions.length}] new keeps...`
      )

      const results = await pAll(actions, {
        concurrency: CONCURRENCY_LIMIT,
      })

      console.log(
          `Successfully fetched information about [${results.length}] new keeps`
      )
    }

    const latestBlockNumber = keepCreatedEvents.slice(-1)[0].blockNumber
    this.cache.assign({ lastRefreshBlock: latestBlockNumber }).write()

    await this.refreshActiveKeeps()
  }

  async fetchKeep(keepData) {
    return new Promise(async (resolve, reject) => {
      try {
        const { address, members, creationBlock } = keepData
        const keepContract = await this.contracts.BondedECDSAKeep.at(address)

        const creationTimestamp = await callWithRetry(
            keepContract.methods.getOpenedTimestamp()
        )

        this.cache
            .get("keeps")
            .push({
              address: address,
              members: members,
              creationBlock: creationBlock,
              creationTimestamp: creationTimestamp,
              status: await getKeepStatus(keepData, this.contracts, this.web3)
            })
            .write()

        console.log(
            `Successfully fetched information about keep ${address}`
        )

        return resolve()
      } catch (err) {
        return reject(err)
      }
    })
  }

  async refreshActiveKeeps() {
    const activeKeeps = this.getKeeps(KeepStatus.ACTIVE)

    console.log(`Refreshing [${activeKeeps.length}] active keeps in the cache`)

    const actions = []
    activeKeeps.forEach(keepData => {
      actions.push(() => this.refreshKeepStatus(keepData))
    })

    await pAll(actions, {concurrency: CONCURRENCY_LIMIT})
  }

  async refreshKeepStatus(keepData) {
    console.log(`Checking current status of keep ${keepData.address}`)

    const lastStatus = keepData.status
    const currentStatus = await getKeepStatus(keepData, this.contracts, this.web3)

    if (lastStatus.name !== currentStatus.name) {
      console.log(
          `Updating current status of keep ${keepData.address} ` +
          `from [${lastStatus.name}] to [${currentStatus.name}]`
      )

      this.cache
          .get("keeps")
          .find({ address: keepData.address })
          .assign({ status: currentStatus })
          .write()
    }
  }

  getKeeps(status) {
    return this.cache
        .get("keeps")
        .filter(keep => !status || keep.status.name === status)
        .value()
  }
}
