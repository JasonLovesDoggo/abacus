import http from 'k6/http';
import {check, sleep} from 'k6';
import {Counter} from 'k6/metrics';

// Custom metrics
const createErrors = new Counter('create_errors');
const hitErrors = new Counter('hit_errors');
const getErrors = new Counter('get_errors');

export const options = {
    executor: 'constant-arrival-rate',
    rate: 1,
    timeUnit: '1s',
    duration: '1m',
    preAllocatedVUs: 5,
    maxVUs: 20,
    // stages: [
    //   { duration: '30s', target: 20 }, // Ramp up to 20 users
    //   { duration: '1m', target: 20 },  // Stay at 20 users
    //   { duration: '30s', target: 0 },  // Ramp down to 0 users
    // ],
    thresholds: {
        http_req_duration: ['p(95)<250'], // 95% of requests should be below 250ms
        create_errors: ['count<10'],
        hit_errors: ['count<10'],
        get_errors: ['count<10'],
    },
};


const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export function setup() {
    // Create a test counter that we'll use throughout the test
    const createResponse = http.post(`${BASE_URL}/create/test/k6-test-counter`);
    check(createResponse, {
        'counter created successfully': (r) => r.status === 201,
    });

    return {
        counterId: 'k6-test-counter',
        adminKey: createResponse.json('admin_key'),
    };
}

export default function (data) {
    // Test counter creation
    const namespace = `test-${__VU}`;
    const key = `counter-${__ITER}`;

    const createResponse = http.post(`${BASE_URL}/create/${namespace}/${key}`);
    if (!check(createResponse, {
        'create status is 201': (r) => r.status === 201,
        'create returns admin_key': (r) => r.json('admin_key') !== undefined,
    })) {
        createErrors.add(1);
    }

    // Test hitting the counter
    const hitResponse = http.get(`${BASE_URL}/hit/${namespace}/${key}`);
    if (!check(hitResponse, {
        'hit status is 200': (r) => r.status === 200,
        'hit returns incremented value': (r) => r.json('value') === 1,
    })) {
        hitErrors.add(1);
    }

    // Test getting the counter value
    const getResponse = http.get(`${BASE_URL}/get/${namespace}/${key}`);
    if (!check(getResponse, {
        'get status is 200': (r) => r.status === 200,
        'get returns correct value': (r) => r.json('value') === 1,
    })) {
        getErrors.add(1);
    }

    // Test counter info
    const infoResponse = http.get(`${BASE_URL}/info/${namespace}/${key}`);
    check(infoResponse, {
        'info status is 200': (r) => r.status === 200,
        'info returns exists true': (r) => r.json('exists') === true,
    });

    // Test updating counter with admin key
    const headers = {
        'Authorization': `Bearer ${createResponse.json('admin_key')}`,
    };
    console.log(headers);

    const updateResponse = http.post(
        `${BASE_URL}/set/${namespace}/${key}?value=10`,
        null,
        {headers}
    );
    check(updateResponse, {
        'update status is 200': (r) => r.status === 200,
        'update returns new value': (r) => r.json('value') === 10,
    });

    sleep(1);
}

export function teardown(data) {
    // Clean up - delete test counters
    const headers = {
        'Authorization': `Bearer ${data.adminKey}`,
    };
    http.post(`${BASE_URL}/delete/test/${data.counterId}`, null, {headers});
}