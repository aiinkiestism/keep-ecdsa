import clc from "cli-color"
import BlockByDate from "ethereum-block-by-date"

import Context from "./lib/context.js"
import FraudDetector from "./lib/fraud-detector.js"
import Requirements from "./lib/requirements.js"
import SLACalculator from "./lib/sla-calculator.js"
import AssetsCalculator from "./lib/assets-calculator.js"
import RewardsCalculator from "./lib/rewards-calculator.js"

async function run() {
  try {
    // URL to the websocket endpoint of the Ethereum node.
    const ethHostname = process.env.ETH_HOSTNAME
    // Start of the interval passed as UNIX timestamp.
    const intervalStart = process.argv[2]
    // End of the interval passed as UNIX timestamp.
    const intervalEnd = process.argv[3]
    // Total KEEP rewards distributed within the given interval.
    const intervalTotalRewards = process.argv[4]
    // Determines whether debug logs should be disabled.
    const isDebugDisabled = process.env.DEBUG !== "on"
    // Determines whether the cache refresh process should be performed.
    // This option should be used only for development purposes. If the
    // cache refresh is disabled, rewards distribution may be calculated
    // based on outdated information from the chain.
    const isCacheRefreshEnabled = process.env.CACHE_REFRESH !== "off"
    // Access key to Tenderly API used to fetch transactions from the chain.
    // Setting it is optional. If not set the script won't call Tenderly API
    // and rely on cached transactions.
    const tenderlyApiKey = process.env.TENDERLY_API_KEY

    if (!ethHostname) {
      console.error(clc.red("Please provide ETH_HOSTNAME value"))
      return
    }

    const interval = {
      start: parseInt(intervalStart),
      end: parseInt(intervalEnd),
      totalRewards: intervalTotalRewards,
    }

    validateIntervalTimestamps(interval)
    validateIntervalTotalRewards(interval)

    if (isDebugDisabled) {
      console.debug = function () {}
    }

    const context = await Context.initialize(ethHostname, tenderlyApiKey)

    await determineIntervalBlockspan(context, interval)

    if (isCacheRefreshEnabled) {
      console.log("Refreshing keeps cache...")
      await context.cache.refresh()
    }

    const operatorsRewards = await calculateOperatorsRewards(context, interval)

    console.table(operatorsRewards)
  } catch (error) {
    throw new Error(error)
  }
}

function validateIntervalTimestamps(interval) {
  const startDate = new Date(interval.start * 1000)
  const endDate = new Date(interval.end * 1000)

  const isValidStartDate = startDate.getTime() > 0
  if (!isValidStartDate) {
    throw new Error("Invalid interval start timestamp")
  }

  const isValidEndDate = endDate.getTime() > 0
  if (!isValidEndDate) {
    throw new Error("Invalid interval end timestamp")
  }

  const isEndAfterStart = endDate.getTime() > startDate.getTime()
  if (!isEndAfterStart) {
    throw new Error(
      "Interval end timestamp should be bigger than the interval start"
    )
  }

  console.log(clc.green(`Interval start timestamp: ${startDate.toISOString()}`))
  console.log(clc.green(`Interval end timestamp: ${endDate.toISOString()}`))
}

function validateIntervalTotalRewards(interval) {
  if (!interval.totalRewards) {
    throw new Error("Interval total rewards should be set")
  }

  console.log(
    clc.green(`Interval total rewards: ${interval.totalRewards} KEEP`)
  )
}

async function determineIntervalBlockspan(context, interval) {
  const blockByDate = new BlockByDate(context.web3)

  interval.startBlock = (await blockByDate.getDate(interval.start * 1000)).block
  interval.endBlock = (await blockByDate.getDate(interval.end * 1000)).block

  console.log(clc.green(`Interval start block: ${interval.startBlock}`))
  console.log(clc.green(`Interval end block: ${interval.endBlock}`))
}

async function calculateOperatorsRewards(context, interval) {
  const { cache } = context

  const fraudDetector = await FraudDetector.initialize(context)
  const requirements = await Requirements.initialize(context, interval)
  const slaCalculator = await SLACalculator.initialize(context, interval)
  const assetsCalculator = await AssetsCalculator.initialize(context, interval)

  await requirements.checkDeauthorizations()

  const operatorsParameters = []

  for (const operator of getOperators(cache)) {
    const isFraudulent = await fraudDetector.isOperatorFraudulent(operator)
    const operatorAuthorizations = await requirements.checkAuthorizations(
      operator
    )
    const operatorSLA = slaCalculator.calculateOperatorSLA(operator)
    const operatorAssets = await assetsCalculator.calculateOperatorAssets(
      operator
    )

    operatorsParameters.push(
      new OperatorParameters(
        operator,
        isFraudulent,
        operatorAuthorizations,
        operatorSLA,
        operatorAssets
      )
    )
  }

  const rewardsCalculator = RewardsCalculator.initialize(
    operatorsParameters,
    interval
  )

  const operatorsSummary = []

  for (const operatorParameters of operatorsParameters) {
    const { operator } = operatorParameters
    const operatorRewards = rewardsCalculator.calculateOperatorRewards(operator)

    operatorsSummary.push(
      new OperatorSummary(
        context.web3,
        operator,
        operatorParameters,
        operatorRewards
      )
    )
  }

  return operatorsSummary
}

// TODO: Change the way operators are fetched. Currently only the ones which
//  have members in existing keeps are taken into account. Instead of that,
//  we should take all operators which are registered in the sorition pool.
function getOperators(cache) {
  const operators = new Set()

  cache
    .getKeeps()
    .forEach((keep) => keep.members.forEach((member) => operators.add(member)))

  return operators
}

function OperatorParameters(
  operator,
  isFraudulent,
  authorizations,
  operatorSLA,
  operatorAssets
) {
  ;(this.operator = operator),
    (this.isFraudulent = isFraudulent),
    (this.authorizations = authorizations)((this.operatorSLA = operatorSLA)),
    (this.operatorAssets = operatorAssets)
}

function OperatorSummary(web3, operator, operatorParameters, operatorRewards) {
  ;(this.operator = operator),
    (this.isFraudulent = operatorParameters.isFraudulent),
    (this.factoryAuthorizedAtStart =
      operatorParameters.authorizations.factoryAuthorizedAtStart),
    (this.poolAuthorizedAtStart =
      operatorParameters.authorizations.poolAuthorizedAtStart),
    (this.poolDeauthorizedInInterval =
      operatorParameters.authorizations.poolDeauthorizedInInterval),
    (this.keygenCount = operatorParameters.operatorSLA.keygenCount),
    (this.keygenFailCount = operatorParameters.operatorSLA.keygenFailCount),
    (this.keygenSLA = operatorParameters.operatorSLA.keygenSLA),
    (this.signatureCount = operatorParameters.operatorSLA.signatureCount),
    (this.signatureFailCount =
      operatorParameters.operatorSLA.signatureFailCount),
    (this.signatureSLA = operatorParameters.operatorSLA.signatureSLA),
    (this.keepStaked = roundKeepValue(
      web3,
      operatorParameters.operatorAssets.keepStaked
    )),
    (this.ethBonded = roundFloat(operatorParameters.operatorAssets.ethBonded)),
    (this.ethUnbonded = roundFloat(
      operatorParameters.operatorAssets.ethUnbonded
    )),
    (this.ethTotal = roundFloat(operatorParameters.operatorAssets.ethTotal)),
    (this.ethScore = operatorRewards.ethScore),
    (this.boost = operatorRewards.boost),
    (this.rewardWeight = operatorRewards.rewardWeight),
    (this.totalRewards = operatorRewards.totalRewards)
}

function roundKeepValue(web3, value) {
  return web3.utils.toBN(value).div(web3.utils.toBN(1e18)).toNumber()
}

function roundFloat(number) {
  return Math.round(number * 100) / 100
}

run()
  .then((result) => {
    console.log(
      clc.green(
        "Staker rewards distribution calculations completed successfully"
      )
    )

    process.exit(0)
  })
  .catch((error) => {
    console.trace(
      clc.red(
        "Staker rewards distribution calculations errored out with error: "
      ),
      error
    )

    process.exit(1)
  })
