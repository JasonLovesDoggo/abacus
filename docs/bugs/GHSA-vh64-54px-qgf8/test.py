#!/usr/bin/env python3
"""
Enhanced test script for Abacus SSE goroutine leak.

This script creates three types of connections:
1. Quick connections that immediately disconnect
2. Lingering connections that stay open for a longer period
3. Zombie connections that are left open until the end of the test

This mixed approach better simulates real-world client behavior and tests
the server's ability to clean up all types of disconnected clients.

Author: JasonLovesDoggo (2025-03-02)
"""

import argparse
import concurrent.futures
import json
import os
import platform
import signal
import sys
import threading
import time
from datetime import datetime
try:
    import psutil
    PSUTIL_AVAILABLE = True
except ImportError:
    PSUTIL_AVAILABLE = False

import requests
import colorama

# Initialize colorama for Windows terminal support
colorama.init()

class AbacusLeakTester:
    def __init__(self, server_url, process_name='abacus',
                 quick_connections=50, lingering_connections=20, zombie_connections=10,
                 num_batches=5, delay_between_batches=5, linger_time=10,
                 timeout=3, max_workers=10):
        self.server_url = server_url.rstrip('/')
        self.endpoint = "/stream/test/leak_test"
        self.process_name = process_name

        # Connection types and counts
        self.quick_connections = quick_connections
        self.lingering_connections = lingering_connections
        self.zombie_connections = zombie_connections

        self.num_batches = num_batches
        self.delay_between_batches = delay_between_batches
        self.linger_time = linger_time
        self.timeout = timeout
        self.max_workers = max_workers
        self.process = None
        self.initial_memory = None

        # Tracking the zombie connections
        self.zombie_sessions = []
        self.zombie_responses = []

        # Results tracking
        self.total_successful = 0
        self.total_failed = 0
        self.memory_readings = []

        # Windows-specific process name adjustments
        if platform.system() == "Windows":
            if not self.process_name.lower().endswith('.exe'):
                self.process_name += '.exe'

    def find_process(self):
        """Find the server process if running locally."""
        if not PSUTIL_AVAILABLE:
            print("psutil module not available. Memory tracking disabled.")
            return None

        for proc in psutil.process_iter(['pid', 'name']):
            try:
                if self.process_name.lower() in proc.info['name'].lower():
                    return proc
            except (psutil.NoSuchProcess, psutil.AccessDenied, psutil.ZombieProcess):
                pass

        return None

    def get_memory_usage(self):
        """Get current memory usage of the process in MB."""
        if not self.process:
            return None

        try:
            # Wait a bit for any memory operations to settle
            time.sleep(1)
            self.process.memory_info()  # Refresh process info
            memory = self.process.memory_info().rss / (1024 * 1024)
            self.memory_readings.append(memory)
            return memory
        except (psutil.NoSuchProcess, psutil.AccessDenied):
            print("Process no longer accessible or has terminated.")
            self.process = None
            return None

    def create_test_counter(self):
        """Create a test counter if it doesn't exist."""
        try:
            response = requests.post(f"{self.server_url}/create/test/leak_test")
            if response.status_code == 201:
                print("\033[92mCreated test counter\033[0m")
            else:
                print(f"\033[93mCounter creation response: {response.status_code}\033[0m")
        except Exception as e:
            print(f"\033[93mCounter may already exist, continuing... ({e})\033[0m")

    def make_quick_connection(self, connection_id, batch_id):
        """Make a connection that immediately disconnects."""
        session = requests.Session()
        try:
            # Start a streaming request
            headers = {"Accept": "text/event-stream"}
            response = session.get(
                f"{self.server_url}{self.endpoint}",
                headers=headers,
                stream=True,
                timeout=self.timeout
            )

            # Just read a tiny bit of data
            try:
                next(response.iter_content(chunk_size=64))
            except (StopIteration, requests.exceptions.ChunkedEncodingError,
                   requests.exceptions.ConnectionError, requests.exceptions.ReadTimeout):
                pass

            # Abruptly close the connection
            response.close()

            # Increment the counter
            hit_response = session.get(f"{self.server_url}/hit/test/leak_test")
            hit_response.raise_for_status()

            return True
        except Exception as e:
            if "Read timed out" not in str(e):  # Ignore expected timeouts
                print(f"Quick connection {connection_id} in batch {batch_id} failed: {str(e)}")
            return False
        finally:
            session.close()

    def make_lingering_connection(self, connection_id, batch_id):
        """Make a connection that stays open for a while before closing."""
        session = requests.Session()
        try:
            # Start a streaming request
            headers = {"Accept": "text/event-stream"}
            response = session.get(
                f"{self.server_url}{self.endpoint}",
                headers=headers,
                stream=True,
                timeout=self.timeout + self.linger_time
            )

            # Read a bit of data
            try:
                for _ in range(2):  # Read a couple of chunks to establish connection
                    next(response.iter_content(chunk_size=128))
            except (StopIteration, requests.exceptions.ChunkedEncodingError):
                pass

            print(f"  Lingering connection {connection_id} established, will stay open for {self.linger_time}s")

            # Keep connection open for a while
            time.sleep(self.linger_time)

            # Properly close the connection
            response.close()

            # Increment the counter
            hit_response = session.get(f"{self.server_url}/hit/test/leak_test")
            hit_response.raise_for_status()

            print(f"  Lingering connection {connection_id} properly closed after {self.linger_time}s")
            return True
        except Exception as e:
            print(f"Lingering connection {connection_id} in batch {batch_id} failed: {str(e)}")
            return False
        finally:
            session.close()

    def make_zombie_connection(self, connection_id, batch_id):
        """Make a connection that is never explicitly closed (until cleanup)."""
        try:
            # Create a persistent session
            session = requests.Session()
            self.zombie_sessions.append(session)

            # Start a streaming request
            headers = {"Accept": "text/event-stream"}
            response = session.get(
                f"{self.server_url}{self.endpoint}",
                headers=headers,
                stream=True,
                timeout=60  # Long timeout
            )
            self.zombie_responses.append(response)

            # Read just a bit to establish the connection
            try:
                next(response.iter_content(chunk_size=64))
            except (StopIteration, requests.exceptions.ChunkedEncodingError):
                return False

            print(f"  Zombie connection {connection_id} established (will remain open)")

            # Increment the counter
            hit_response = requests.get(f"{self.server_url}/hit/test/leak_test")
            hit_response.raise_for_status()

            return True
        except Exception as e:
            print(f"Zombie connection {connection_id} in batch {batch_id} failed: {str(e)}")
            return False

    def cleanup_zombie_connections(self):
        """Clean up any zombie connections at the end of the test."""
        print("\n\033[93mCleaning up zombie connections...\033[0m")
        for i, response in enumerate(self.zombie_responses):
            try:
                response.close()
                print(f"  Closed zombie connection {i+1}")
            except:
                pass

        for i, session in enumerate(self.zombie_sessions):
            try:
                session.close()
                print(f"  Closed zombie session {i+1}")
            except:
                pass

        # Clear the lists
        self.zombie_responses = []
        self.zombie_sessions = []

    def run_batch(self, batch_id):
        """Run a batch with different connection types."""
        print(f"\033[95mStarting batch {batch_id} of {self.num_batches}\033[0m")

        batch_start = time.time()
        batch_successful = 0
        batch_failed = 0

        # 1. Quick connections (in parallel)
        if self.quick_connections > 0:
            print(f"  Creating {self.quick_connections} quick connections...")
            with concurrent.futures.ThreadPoolExecutor(max_workers=self.max_workers) as executor:
                futures = []
                for i in range(1, self.quick_connections + 1):
                    futures.append(executor.submit(self.make_quick_connection, i, batch_id))
                    if i % 20 == 0:
                        time.sleep(0.2)  # Stagger connections

                for future in concurrent.futures.as_completed(futures):
                    if future.result():
                        batch_successful += 1
                    else:
                        batch_failed += 1

        # 2. Lingering connections (in parallel but with careful thread management)
        if self.lingering_connections > 0:
            print(f"  Creating {self.lingering_connections} lingering connections...")
            # Use a smaller thread pool to avoid overwhelming resources
            max_lingering_threads = min(5, self.max_workers)
            with concurrent.futures.ThreadPoolExecutor(max_workers=max_lingering_threads) as executor:
                futures = []
                for i in range(1, self.lingering_connections + 1):
                    futures.append(executor.submit(self.make_lingering_connection, i, batch_id))
                    time.sleep(0.5)  # Stagger lingering connections more

                for future in concurrent.futures.as_completed(futures):
                    if future.result():
                        batch_successful += 1
                    else:
                        batch_failed += 1

        # 3. Zombie connections (create but don't close)
        if batch_id == 1 and self.zombie_connections > 0:  # Only create zombies in first batch
            print(f"  Creating {self.zombie_connections} zombie connections...")
            for i in range(1, self.zombie_connections + 1):
                if self.make_zombie_connection(i, batch_id):
                    batch_successful += 1
                else:
                    batch_failed += 1
                time.sleep(0.5)  # Delay between zombie connections

        self.total_successful += batch_successful
        self.total_failed += batch_failed

        batch_duration = time.time() - batch_start
        print(f"\033[92mBatch {batch_id} completed: {batch_successful} successful, "
              f"{batch_failed} failed. Duration: {batch_duration:.2f}s\033[0m")

        return batch_successful, batch_failed

    def check_memory(self):
        """Check and report on memory usage."""
        if not self.process:
            return True

        current_memory = self.get_memory_usage()
        if current_memory is None:
            print("\033[91mProcess no longer found. Server may have crashed!\033[0m")
            return False

        memory_diff = current_memory - self.initial_memory
        color = "\033[96m"  # Cyan
        if memory_diff > 15:
            color = "\033[91m"  # Red
        elif memory_diff > 5:
            color = "\033[93m"  # Yellow

        sign = "+" if memory_diff >= 0 else ""
        print(f"{color}Current memory: {current_memory:.2f} MB ({sign}{memory_diff:.2f} MB)\033[0m")
        return True

    def print_final_stats(self):
        """Print final test statistics."""
        final_memory = None
        if self.process:
            final_memory = self.get_memory_usage()

        print("\n\033[95mTest completed!\033[0m")
        total_connections = (
            (self.quick_connections * self.num_batches) +
            (self.lingering_connections * self.num_batches) +
            self.zombie_connections
        )
        print(f"\033[97mTotal connections attempted: {total_connections}\033[0m")
        print(f"  Quick connections: {self.quick_connections * self.num_batches}")
        print(f"  Lingering connections: {self.lingering_connections * self.num_batches}")
        print(f"  Zombie connections: {self.zombie_connections}")

        print(f"\033[92mSuccessful: {self.total_successful}\033[0m")
        color = "\033[92m" if self.total_failed == 0 else "\033[91m"
        print(f"{color}Failed: {self.total_failed}\033[0m")

        if self.initial_memory is not None and len(self.memory_readings) > 1:
            print(f"\n\033[96mMemory Analysis:\033[0m")
            print(f"\033[97mInitial memory: {self.initial_memory:.2f} MB\033[0m")
            print(f"\033[97mFinal memory: {final_memory:.2f} MB\033[0m")

            memory_growth = final_memory - self.initial_memory
            growth_percent = (memory_growth / self.initial_memory) * 100

            sign = "+" if memory_growth >= 0 else ""
            color = "\033[92m"  # Green
            if growth_percent > 20:
                color = "\033[91m"  # Red
            elif growth_percent > 10:
                color = "\033[93m"  # Yellow

            print(f"{color}Growth: {sign}{memory_growth:.2f} MB ({sign}{growth_percent:.2f}%)\033[0m")

            # Check for consistent growth pattern
            if len(self.memory_readings) >= 3:
                print("\n\033[96mMemory Growth Pattern:\033[0m")
                for i in range(1, len(self.memory_readings)):
                    diff = self.memory_readings[i] - self.memory_readings[i-1]
                    print(f"  Batch {i}: {self.memory_readings[i]:.2f} MB ({'+' if diff >= 0 else ''}{diff:.2f} MB)")

                # Check for leak indicators
                consistent_growth = True
                baseline_diff = self.memory_readings[1] - self.memory_readings[0]
                for i in range(2, len(self.memory_readings)):
                    diff = self.memory_readings[i] - self.memory_readings[i-1]
                    # If growth is inconsistent (allowing for some variance)
                    if diff < 0 or abs(diff - baseline_diff) > max(baseline_diff * 0.5, 1.0):
                        consistent_growth = False

                if memory_growth > 10 and consistent_growth:
                    print(f"\033[91mConsistent memory growth detected across batches!")
                    print(f"This strongly indicates a memory/goroutine leak.\033[0m")
                elif memory_growth > 10:
                    print(f"\033[93mSignificant memory growth detected but pattern is inconsistent.")
                    print(f"This may indicate a partial leak or normal memory variation.\033[0m")
                elif memory_growth > 5:
                    print(f"\033[93mModerate memory growth detected. May be normal variation.\033[0m")
                else:
                    print(f"\033[92mMemory usage appears stable. No obvious leak detected.\033[0m")

    def get_final_counter(self):
        """Get the final counter value."""
        try:
            response = requests.get(f"{self.server_url}/get/test/leak_test")
            counter_value = response.json().get('value', 'unknown')
            print(f"\n\033[96mCounter value after test: {counter_value}\033[0m")
        except Exception as e:
            print(f"\n\033[91mCould not get final counter value: {e}\033[0m")

    def run_test(self):
        """Run the complete test."""
        print(f"Testing Abacus SSE endpoint at {self.server_url}")
        print(f"Running on {platform.system()} {platform.release()}")
        print(f"Connection configuration:")
        print(f"  - Quick connections: {self.quick_connections}/batch")
        print(f"  - Lingering connections: {self.lingering_connections}/batch (stay open for {self.linger_time}s)")
        print(f"  - Zombie connections: {self.zombie_connections} (left open until end)")
        print(f"  - Total batches: {self.num_batches}")

        # Find process for memory tracking
        self.process = self.find_process()
        if self.process:
            self.initial_memory = self.get_memory_usage()
            print(f"\033[96mInitial memory usage: {self.initial_memory:.2f} MB\033[0m")
        else:
            print("\033[93mCould not find local process. Memory tracking disabled.\033[0m")

        # Create the counter
        self.create_test_counter()

        try:
            # Run test batches
            for batch in range(1, self.num_batches + 1):
                self.run_batch(batch)

                # Check memory
                if self.process and not self.check_memory():
                    break

                # Delay between batches
                if batch < self.num_batches:
                    print(f"\033[90mWaiting {self.delay_between_batches} seconds before next batch...\033[0m")
                    time.sleep(self.delay_between_batches)

            # After all batches, wait a bit longer to see if memory stabilizes
            print("\n\033[93mWaiting 10 seconds for memory to stabilize...\033[0m")
            time.sleep(10)

            # Final memory check
            if self.process:
                self.check_memory()

            # Print statistics
            self.print_final_stats()
            self.get_final_counter()

        finally:
            # Always clean up zombie connections
            self.cleanup_zombie_connections()


def main():
    parser = argparse.ArgumentParser(description='Test for goroutine leaks in Abacus SSE implementation')
    parser.add_argument('--url', default='http://localhost:8080', help='Abacus server URL')
    parser.add_argument('--process', default='abacus', help='Process name for memory tracking')
    parser.add_argument('--quick', type=int, default=50, help='Quick connections per batch')
    parser.add_argument('--lingering', type=int, default=20, help='Lingering connections per batch')
    parser.add_argument('--zombie', type=int, default=10, help='Total zombie connections')
    parser.add_argument('--batches', type=int, default=5, help='Number of batches')
    parser.add_argument('--delay', type=int, default=5, help='Delay between batches (seconds)')
    parser.add_argument('--linger', type=int, default=10, help='How long lingering connections stay open (seconds)')
    parser.add_argument('--workers', type=int, default=10, help='Max concurrent connections')

    args = parser.parse_args()

    # Platform-specific adjustments
    if platform.system() == "Windows":
        if args.workers > 10:
            args.workers = 10
        if args.quick > 30:
            args.quick = 30

    tester = AbacusLeakTester(
        server_url=args.url,
        process_name=args.process,
        quick_connections=args.quick,
        lingering_connections=args.lingering,
        zombie_connections=args.zombie,
        num_batches=args.batches,
        delay_between_batches=args.delay,
        linger_time=args.linger,
        max_workers=args.workers
    )

    try:
        tester.run_test()
    except KeyboardInterrupt:
        print("\n\033[93mTest interrupted by user.\033[0m")
        # Clean up even on keyboard interrupt
        tester.cleanup_zombie_connections()
    finally:
        colorama.deinit()


if __name__ == '__main__':
    main()
