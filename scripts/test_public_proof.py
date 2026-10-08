import importlib.util
from pathlib import Path
import unittest
import xml.etree.ElementTree as ET

spec = importlib.util.spec_from_file_location('proof', Path(__file__).with_name('render-public-proof-badge.py'))
proof = importlib.util.module_from_spec(spec)
spec.loader.exec_module(proof)


class ProofTests(unittest.TestCase):
    def render(self, jobs):
        return proof.render({'id': 123, 'run_attempt': 2, 'head_sha': 'a'*40,
            'html_url': 'https://github.com/angusu-de/ByeClaude/actions/runs/123',
            'status': 'completed', 'conclusion': 'success'}, jobs)

    def test_missing_jobs_are_never_counted_as_passed(self):
        svg, data, _ = self.render([])
        self.assertEqual(data['jobs_success'], 0)
        self.assertEqual(data['jobs_total'], 10)
        self.assertIn('0/10', svg)
        ET.fromstring(svg)

    def test_actual_steps_and_mixed_outcomes_are_preserved(self):
        jobs = [{'name': name, 'status':'completed', 'conclusion':'success', 'steps':[
            {'name':'test <input> & output', 'status':'completed', 'conclusion':'success'}]} for name in proof.EXPECTED]
        jobs[1]['conclusion'] = 'failure'
        jobs[2]['conclusion'] = 'skipped'
        svg, data, readme = self.render(jobs)
        self.assertEqual(data['jobs_success'], 8)
        self.assertIn('proof-failure', svg)
        self.assertEqual(data['jobs'][0]['steps'], jobs[0]['steps'])
        self.assertIn('test &lt;input&gt; &amp; output', readme)
        self.assertIn('attempt 2', readme)
        ET.fromstring(svg)

    def test_duplicate_and_running_jobs_cannot_inflate_success(self):
        job = {'name':proof.EXPECTED[0], 'status':'in_progress', 'conclusion':'success'}
        self.assertEqual(self.render([job])[1]['jobs_success'], 0)
        with self.assertRaises(ValueError): self.render([job, job])


if __name__ == '__main__': unittest.main()
