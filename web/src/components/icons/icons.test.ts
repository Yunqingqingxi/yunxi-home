import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import AgentStateIcon from './AgentStateIcon.vue'
import RoleIcon from './RoleIcon.vue'
import StatusDot from './StatusDot.vue'

const allStates = [
  'start', 'reasoning', 'executing', 'waiting_lock',
  'waiting_human', 'delegate', 'suspended', 'timeout',
  'retry', 'done', 'failed', 'cancel',
]

describe('AgentStateIcon', () => {
  it.each(allStates)('renders SVG for state: %s', (state) => {
    const wrapper = mount(AgentStateIcon, { props: { state: state as any } })
    expect(wrapper.find('svg').exists()).toBe(true)
    expect(wrapper.classes()).toContain(`state-${state}`)
    wrapper.unmount()
  })

  it('has spin animation for executing and retry', () => {
    for (const state of ['executing', 'retry']) {
      const wrapper = mount(AgentStateIcon, { props: { state: state as any } })
      expect(wrapper.classes()).toContain('icon-spin')
      wrapper.unmount()
    }
  })

  it('has pulse animation for reasoning', () => {
    const wrapper = mount(AgentStateIcon, { props: { state: 'reasoning' } })
    expect(wrapper.classes()).toContain('icon-pulse')
    wrapper.unmount()
  })

  it('has blink animation for waiting_human', () => {
    const wrapper = mount(AgentStateIcon, { props: { state: 'waiting_human' } })
    expect(wrapper.classes()).toContain('icon-blink')
    wrapper.unmount()
  })

  it('no animation for done, failed, cancel', () => {
    for (const state of ['done', 'failed', 'cancel', 'start', 'suspended']) {
      const wrapper = mount(AgentStateIcon, { props: { state: state as any } })
      expect(wrapper.classes()).not.toContain('icon-spin')
      expect(wrapper.classes()).not.toContain('icon-pulse')
      expect(wrapper.classes()).not.toContain('icon-blink')
      wrapper.unmount()
    }
  })

  it('accepts custom size prop', () => {
    const wrapper = mount(AgentStateIcon, { props: { state: 'done', size: 32 } })
    const svg = wrapper.find('svg')
    expect(svg.attributes('width')).toBe('32')
    expect(svg.attributes('height')).toBe('32')
    wrapper.unmount()
  })

  it('default size is 20', () => {
    const wrapper = mount(AgentStateIcon, { props: { state: 'done' } })
    const svg = wrapper.find('svg')
    expect(svg.attributes('width')).toBe('20')
    wrapper.unmount()
  })
})

describe('RoleIcon', () => {
  const roles = ['executor', 'supervisor', 'manager']

  it.each(roles)('renders SVG for role: %s', (role) => {
    const wrapper = mount(RoleIcon, { props: { role: role as any } })
    expect(wrapper.find('svg').exists()).toBe(true)
    expect(wrapper.classes()).toContain(`role-${role}`)
    wrapper.unmount()
  })

  it('accepts custom size', () => {
    const wrapper = mount(RoleIcon, { props: { role: 'supervisor', size: 24 } })
    expect(wrapper.find('svg').attributes('width')).toBe('24')
    wrapper.unmount()
  })
})

describe('StatusDot', () => {
  it('renders dot element with correct class', () => {
    const wrapper = mount(StatusDot, { props: { status: 'done' } })
    expect(wrapper.find('.dot-done').exists()).toBe(true)
    wrapper.unmount()
  })

  it('shows tooltip text', () => {
    const wrapper = mount(StatusDot, { props: { status: 'running', tooltip: '执行中' } })
    expect(wrapper.attributes('title')).toBe('执行中')
    wrapper.unmount()
  })

  it.each(['running', 'done', 'failed', 'waiting_lock', 'cancel'])('renders dot for %s', (s) => {
    const wrapper = mount(StatusDot, { props: { status: s } })
    expect(wrapper.find('.dot').exists()).toBe(true)
    wrapper.unmount()
  })
})
