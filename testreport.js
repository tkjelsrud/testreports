const xmlbuilder = require('xmlbuilder');

class TestCase {
    constructor(name) {
        this.name = name;
        this.result = 'failed'; // Default to failed unless specified
        this.duration = 0; // Duration in seconds
        this.startTime = new Date();
        this.errorText = '';
    }

    updateResult(result, errorText = '') {
        this.result = result;
        // Hack, set duration when result is known
        this.duration = new Date() - this.startTime;
        this.errorText = errorText;
    }

    updateDuration(duration) {
        this.duration = duration;
    }

    toJUnitXML() {
        const testCase = {
            'testcase': {
                '@name': this.name,
                '@time': this.duration,
                '@assertions': 1,
            }
        };

        if (this.result !== 'passed') {
            testCase.testcase.failure = {
                '@message': 'Test failed',
                '#text': this.errorText
            };
        }

        return testCase;
    }
}

class TestSuite {
    constructor(name) {
        this.name = name;
        this.testCases = [];
        this.date = new Date();
    }

    addTestCase(testCase) {
        this.testCases.push(testCase);
    }

    toJUnitXML() {
        const suite = {
            'testsuite': {
                '@name': this.name,
                '@tests': this.testCases.length,
                '@failures': this.testCases.filter(tc => tc.result == 'failed').length,
                '@skipped': this.testCases.filter(tc => tc.result == 'skipped').length,
                '@timestamp': this.date.toISOString(), //ISO 8601 
                'testcase': this.testCases.map(tc => tc.toJUnitXML().testcase)
            }
        };

        return suite;
    }
}

class TestReport {
    constructor() {
        this.suites = [];
    }

    addSuite(suite) {
        this.suites.push(suite);
    }

    toJUnitXML() {
        const xmlObj = {
            'testsuites': {
                'testsuite': this.suites.map(suite => suite.toJUnitXML().testsuite)
            }
        };

        return xmlbuilder.create(xmlObj, { encoding: 'UTF-8' }).end({ pretty: true });
    }
}

class ApdexSummary {
    constructor(tresholdSec = 1.0) {
        this.tresholdMs = tresholdSec * 1000;
        this.counters = [];
    }

    static Level = Object.freeze({
        SATISFIED: 'Satisfied (T)',
        TOLERATING: 'Tolerating (4T)',
        FRUSTRATED: 'Frustrated (>4T)'
    });

    start(label) {
        this.counters.push({'label': label, 'start': performance.now(), 'end': null, 'assertion': true});
    }

    stop(assertion = true) {
        this.counters[this.counters.length - 1].end = performance.now();
        this.counters[this.counters.length - 1].assertion = assertion;
    }

    duration(i) {
        if(this.counters[i].end != null)
            return this.counters[i].end - this.counters[i].start;

        return null;
    }

    length() {
        return this.counters.length;
    }

    score() {
        // Score myself
    }

    scoreArray(durations) {
        // static, score an array
        let s = 0;
        let t = 0;

        for(let i = 0; i < durations.length; i++) {
            if(this.apdex(durations[i]) == ApdexSummary.Level.SATISFIED) s++;
            if(this.apdex(durations[i]) == ApdexSummary.Level.TOLERATING) t++;
        }

        return "Apdex = (" + s + " + (" + t + "/2) ) / " + durations.length + " = " + ((s + (t/2)) / durations.length);
    }

    apdex(duration) {
        if(duration != null) {
            if(duration <= this.tresholdMs)
                return ApdexSummary.Level.SATISFIED;
            if(duration <= (this.tresholdMs * 4))
                return ApdexSummary.Level.TOLERATING;

            return ApdexSummary.Level.FRUSTRATED;
        }
        return null;
    }

    toString() {
        let str = "";
        for(let i = 0; i < this.counters.length; i++) {
            str = str + this.counters[i].label + ": " + Math.round(this.duration(i) / 1000) + " " + this.apdex(this.duration(i)) + " " + (this.counters[i].assertion ? 'passed': 'failed') + "\n";
        }

        return str;
    }
}

function unit_test() {
    // Example Usage
    const report = new TestReport();
    const uiTests = new TestSuite('UI Tests');
    uiTests.addTestCase(new TestCase('Login Test'));
    uiTests.addTestCase(new TestCase('Logout Test'));

    uiTests.testCases[0].updateResult('passed');
    uiTests.testCases[0].updateDuration(120);
    uiTests.testCases[1].updateResult('failed');
    uiTests.testCases[1].updateDuration(150);

    report.addSuite(uiTests);

    // Generate JUnit XML
    console.log(report.toJUnitXML());
}

module.exports = { TestCase, TestSuite, TestReport, ApdexSummary };
