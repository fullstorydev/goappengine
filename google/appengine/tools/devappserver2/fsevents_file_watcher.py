#!/usr/bin/env python
#
# Replacement `MtimeFileWatcher` for App Engine SDK's dev_appserver.py,
# designed for OS X. Improves upon existing file watcher (under OS X) in
# numerous ways:
#
#   - Uses FSEvents API to watch for changes instead of polling. This saves a
#     dramatic amount of CPU, especially in projects with several modules.
#   - Tries to be smarter about which modules reload when files change, only
#     modified module should reload.
#
# Install:
#   $ pip install macfsevents
#   $ cp mtime_file_watcher.py \
#        sdk/google/appengine/tools/devappserver2/mtime_file_watcher.py
import os
import time
#from os.path import abspath, join

try:
  from fsevents import Observer
  from fsevents import Stream
except ImportError:
  Observer = None
  Stream = None

class FSEventsFileWatcher(object):
    SUPPORTS_MULTIPLE_DIRECTORIES = True

    def __init__(self, directories, **kwargs):
        self._changes = _changes = []
        self.observer = Observer()

        def callback(event, mask=None):
            _changes.append(event.name)

        self.stream = Stream(callback, file_events=True, *directories)

    @staticmethod
    def is_available():
      return Observer is not None

    def start(self):
        self.observer.schedule(self.stream)
        self.observer.start()

    def changes(self, timeout=None):
        time.sleep(0.1)
        changed = set(self._changes)
        del self._changes[:]
        return changed

    def quit(self):
        self.observer.unschedule(self.stream)
        self.observer.stop()
        self.observer.join()
