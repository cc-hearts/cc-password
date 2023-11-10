import { classnames } from '@/utils/classnames';
import { describe, test, expect } from 'vitest';
describe('classnames', () => {
  test('returns empty string when no arguments', () => {
    expect(classnames()).toBe('');
  });

  test('returns single string when one string argument', () => {
    expect(classnames('test')).toBe('test');
  });

  test('returns space-separated string when multiple string arguments', () => {
    expect(classnames('test', 'case')).toBe('test case');
  });

  test('ignores boolean arguments', () => {
    expect(classnames('test', false, 'case')).toBe('test case');
  });

  test('returns stringified number when number argument', () => {
    expect(classnames('test', 123, 'case')).toBe('test 123 case');
  });

  test('returns space-separated string when array argument', () => {
    expect(classnames('test', ['case', 'study'])).toBe('test case study');
  });

  test('returns keys with truthy values when object argument', () => {
    expect(classnames('test', { case: true, study: false })).toBe('test case');
  });
});