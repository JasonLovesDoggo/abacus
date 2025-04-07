import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter } from 'k6/metrics';

// Custom metrics
const hitErrors = new Counter('hit_errors');
const getErrors = new Counter('get_errors');

export const options = {
  vus: 10,
  duration: '30s',
  thresholds: {
    http_req_duration: ['p(95)<250'], // 95% of requests should be below 250ms
    hit_errors: ['count<10'],
    get_errors: ['count<10'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'https://abacus.jasoncameron.dev';

// Helper function for safer JSON parsing
function safeGetValue(response, key) {
  try {
    if (!response.body) return undefined;
    const data = JSON.parse(response.body);
    return data[key];
  } catch (e) {
    return undefined;
  }
}

export function setup() {
  console.log(`Running performance tests against ${BASE_URL} at ${new Date().toISOString()}`);

  // Create a test counter
  const createResponse = http.post(`${BASE_URL}/create/test/k6-test-counter`);

  // 201 = Created, 409 = Already exists (both are fine for our purpose)
  check(createResponse, {
    'counter created or exists': (r) => r.status === 201 || r.status === 409,
  });

  // If 201 Created, get admin key, otherwise try to continue without it
  let adminKey = null;
  if (createResponse.status === 201) {
    adminKey = safeGetValue(createResponse, 'admin_key');
  }

  return {
    counterId: 'k6-test-counter',
    adminKey: adminKey
  };
}

export default function (data) {
  // Use a smaller set of test counters
  const keyIndex = (__VU * 100 + __ITER) % 10;
  const key = `counter-${keyIndex}`;
  const namespace = 'test';
  const fullKey = `${namespace}/${key}`;

  // CREATE - 201 Created or 409 Conflict are both acceptable outcomes
  const createResponse = http.post(`${BASE_URL}/create/${fullKey}`);

  check(createResponse, {
    'counter created or already exists': (r) => r.status === 201 || r.status === 409,
  });

  // Get admin key if the counter was just created
  let adminKey = null;
  if (createResponse.status === 201) {
    adminKey = safeGetValue(createResponse, 'admin_key');
  }

  // HIT - increment counter
  const hitResponse = http.get(`${BASE_URL}/hit/${fullKey}`);

  if (!check(hitResponse, {
    'hit successful': (r) => r.status === 200,
  })) {
    hitErrors.add(1);
  }

  // GET - retrieve value
  const getResponse = http.get(`${BASE_URL}/get/${fullKey}`);

  if (!check(getResponse, {
    'get successful': (r) => r.status === 200,
  })) {
    getErrors.add(1);
  }

  // Only do admin operations if we have an admin key from this iteration
  // or occasionally for established counters
  if (adminKey || (__ITER % 5 === 0)) {
    // INFO - check counter info
    http.get(`${BASE_URL}/info/${fullKey}`);

    // UPDATE - set counter value (only if we have an admin key)
    if (adminKey) {
      const headers = { 'Authorization': `Bearer ${adminKey}` };
      http.post(
        `${BASE_URL}/set/${fullKey}?value=10`,
        null,
        { headers }
      );
    }
  }
}

export function teardown(data) {
  // Only attempt cleanup if we have an admin key
  if (data && data.adminKey) {
    const headers = { 'Authorization': `Bearer ${data.adminKey}` };
    http.post(
      `${BASE_URL}/delete/test/${data.counterId}`,
      null,
      { headers }
    );
  }
}
