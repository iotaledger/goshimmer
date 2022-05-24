/**
 * Creating a sidebar enables you to:
 - create an ordered group of docs
 - render a sidebar for each doc of that group
 - provide next/previous navigation

 The sidebars can be generated from the filesystem, or explicitly defined here.

 Create as many sidebars as you want.
 */

module.exports = {
  // But you can create a sidebar manually
  docs: [{
    type: 'doc',
    label: 'Welcome',
    id: 'welcome',
  },
  {
    type: 'doc',
    label: 'FAQ',
    id: 'faq',
  },
  {
    type: 'category',
    label: 'Tutorials',
    items: [{
      type: 'doc',
      label: 'Set up a node',
      id: 'tutorials/setup',
    },
    {
      type: 'doc',
      label: 'Obtain tokens',
      id: 'tutorials/obtain_tokens',
    },
    {
      type: 'doc',
      label: 'Wallet library',
      id: 'tutorials/wallet_library',
    },

    {
      type: 'doc',
      label: 'Write a dApp',
      id: 'tutorials/dApp',
    },

    {
      type: 'doc',
      label: 'Manual peering',
      id: 'tutorials/manual_peering',
    },

    {
      type: 'doc',
      label: 'Create a static identity',
      id: 'tutorials/static_identity',
    },

    {
      type: 'doc',
      label: 'Set up a custom dRNG committee',
      id: 'tutorials/custom_dRNG',
    },

    {
      type: 'doc',
      label: 'Set up the Monitoring Dashboard',
      id: 'tutorials/monitoring',
    },

    {
      type: 'doc',
      label: 'How to create and send transactions',
      id: 'tutorials/send_transaction',
    },
    ],
  },


  {
    type: 'category',
    label: 'Implementation design',
    items: [
      {
        type: 'doc',
        label: 'Event driven model',
        id: 'implementation_design/event_driven_model',
      },
      {
        type: 'doc',
        label: 'Packages and plugins',
        id: 'implementation_design/packages_plugins',
      },

      {
        type: 'doc',
        label: 'Plugin',
        id: 'implementation_design/plugin',
      },

      {
        type: 'doc',
        label: 'Configuration parameters',
        id: 'implementation_design/configuration_parameters',
      },

      {
        type: 'doc',
        label: 'Object storage',
        id: 'implementation_design/object_storage',
      },
    ],
  },
  {
    type: 'category',
    label: 'Protocol Specification',
    items: [
      {
        type: 'doc',
        label: 'Protocol Specification',
        id: 'protocol_specification/overview',
      },
      {
        type: 'doc',
        label: 'Protocol High Level Overview',
        id: 'protocol_specification/protocol',
      },

      {
        type: 'category',
        label: 'Components',
        items: [


          {
            type: 'doc',
            label: 'Overview',
            id: 'protocol_specification/components/overview',
          },

          {
            type: 'doc',
            label: 'Tangle',
            id: 'protocol_specification/components/tangle',
          },

          {
            type: 'doc',
            label: 'Autopeering',
            id: 'protocol_specification/components/autopeering',
          },

          {
            type: 'doc',
            label: 'Mana',
            id: 'protocol_specification/components/mana',
          },

          {
            type: 'doc',
            label: 'Congestion Control',
            id: 'protocol_specification/components/congestion_control',
          },

          {
            type: 'doc',
            label: 'Consensus Mechanism',
            id: 'protocol_specification/components/consensus_mechanism',
          },

          {
            type: 'doc',
            label: 'UTXO and Ledgerstate',
            id: 'protocol_specification/components/ledgerstate',
          },

          {
            type: 'doc',
            label: 'Advanced Outputs (Experimental)',
            id: 'protocol_specification/components/advanced_outputs',
          },

          {
            type: 'doc',
            label: 'Markers',
            id: 'protocol_specification/components/markers',
          },
        ]
      },
      {
        type: 'doc',
        label: 'Glossary',
        id: 'protocol_specification/glossary',
      },
    ]
  },
  {
    type: 'category',
    label: 'API',
    items: [

      {
        type: 'doc',
        label: 'Client Lib',
        id: 'apis/client_lib',
      },

      {
        type: 'doc',
        label: 'WebAPI',
        id: 'apis/webAPI',
      },

      {
        type: 'doc',
        label: 'Node Info',
        id: 'apis/info',
      },

      {
        type: 'doc',
        label: 'Autopeering',
        id: 'apis/autopeering',
      },

      {
        type: 'doc',
        label: 'Manual Peering',
        id: 'apis/manual_peering',
      },

      {
        type: 'doc',
        label: 'Communication Layer',
        id: 'apis/communication',
      },

      {
        type: 'doc',
        label: 'Ledgerstate',
        id: 'apis/ledgerstate',
      },

      {
        type: 'doc',
        label: 'Mana',
        id: 'apis/mana',
      },

      {
        type: 'doc',
        label: 'dRNG',
        id: 'apis/dRNG',
      },

      {
        type: 'doc',
        label: 'Snapshot',
        id: 'apis/snapshot',
      },

      {
        type: 'doc',
        label: 'Faucet',
        id: 'apis/faucet',
      },

      {
        type: 'doc',
        label: 'Spammer',
        id: 'apis/spammer',
      },

      {
        type: 'doc',
        label: 'Tools',
        id: 'apis/tools',
      },
    ],
  },
  {
    type: 'category',
    label: 'Tooling',
    items: [

      {
        type: 'doc',
        label: 'Overview',
        id: 'tooling/overview',
      },

      {
        type: 'doc',
        label: 'Docker Private Network',
        id: 'tooling/docker_private_network',
      },

      {
        type: 'doc',
        label: 'Integration Tests',
        id: 'tooling/integration_tests',
      },

      {
        type: 'doc',
        label: 'DAGs Visualizer',
        id: 'tooling/dags_visualizer',
      },

      {
        type: 'doc',
        label: 'Evil Spammer',
        id: 'tooling/evil_spammer',
      },
    ],
  },
  {
    type: 'category',
    label: 'Team Resources',
    items: [

      {
        type: 'doc',
        label: 'How To Do a Release',
        id: 'teamresources/release',
      },

      {
        type: 'doc',
        label: 'Code Guidelines',
        id: 'teamresources/guidelines',
      },

      {
        type: 'doc',
        label: 'Local Development',
        id: 'teamresources/local_development',
      },

      {
        type: 'doc',
        label: 'Modify the Analysis Dashboard',
        id: 'teamresources/analysis_dashboard',
      },
    ],
  },
  {
    type: 'link',
    label: 'Release Notes',
    href: 'https://github.com/iotaledger/goshimmer/releases'
  }
  ],
};
